package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	acceptedMaxDynamicReactorsPerSession = 4
	acceptedMaxResidentWorkersPerSet     = 3
)

var (
	ErrReactorTelemetrySessionCap = errors.New("reactor telemetry session cap reached")
	ErrReactorTelemetryNotFound   = errors.New("reactor telemetry worker set not found")
)

type ReactorTelemetryLifecycle string

const (
	ReactorTelemetryStarting      ReactorTelemetryLifecycle = "starting"
	ReactorTelemetryActive        ReactorTelemetryLifecycle = "active"
	ReactorTelemetryLaunchFailed  ReactorTelemetryLifecycle = "launch-failed"
	ReactorTelemetryCleanup       ReactorTelemetryLifecycle = "cleanup"
	ReactorTelemetryCleanupFailed ReactorTelemetryLifecycle = "cleanup-failed"
	ReactorTelemetryRemoved       ReactorTelemetryLifecycle = "removed"
)

type ReactorTelemetryConfig struct {
	Enabled               bool
	ControlStore          string
	PostgresDSN           string
	Runtime               string
	WorkerImage           string
	WorkerNetwork         string
	MaxReactorsPerSession int
	WorkersPerSet         int
	SessionTTL            time.Duration
	MeasuredRetention     time.Duration
	CleanupTimeout        time.Duration
	ReconcileInterval     time.Duration
	IngestBaseURL         string
	CredentialSigningKey  string
}

func DefaultReactorTelemetryConfig() ReactorTelemetryConfig {
	return ReactorTelemetryConfig{
		ControlStore:          "memory",
		Runtime:               "contract",
		WorkerImage:           "radiant-scada-standins:latest",
		WorkerNetwork:         "bridge",
		MaxReactorsPerSession: acceptedMaxDynamicReactorsPerSession,
		WorkersPerSet:         acceptedMaxResidentWorkersPerSet,
		SessionTTL:            24 * time.Hour,
		MeasuredRetention:     24 * time.Hour,
		CleanupTimeout:        5 * time.Minute,
		ReconcileInterval:     30 * time.Second,
		IngestBaseURL:         "http://127.0.0.1:8080",
		CredentialSigningKey:  "local-reactor-telemetry-signing-key",
	}
}

func (c ReactorTelemetryConfig) Validate() error {
	if c.MaxReactorsPerSession < 1 || c.MaxReactorsPerSession > acceptedMaxDynamicReactorsPerSession {
		return fmt.Errorf("reactor telemetry max reactors per session must be between 1 and %d", acceptedMaxDynamicReactorsPerSession)
	}
	if c.WorkersPerSet < 1 || c.WorkersPerSet > acceptedMaxResidentWorkersPerSet {
		return fmt.Errorf("reactor telemetry workers per set must be between 1 and %d", acceptedMaxResidentWorkersPerSet)
	}
	if c.SessionTTL <= 0 || c.SessionTTL > 24*time.Hour {
		return fmt.Errorf("reactor telemetry session TTL must be positive and no greater than 24 hours")
	}
	if c.MeasuredRetention <= 0 || c.MeasuredRetention > 24*time.Hour {
		return fmt.Errorf("reactor telemetry measured retention must be positive and no greater than 24 hours")
	}
	if c.CleanupTimeout <= 0 || c.CleanupTimeout > 5*time.Minute {
		return fmt.Errorf("reactor telemetry cleanup timeout must be positive and no greater than five minutes")
	}
	if c.ReconcileInterval <= 0 || c.ReconcileInterval > time.Minute {
		return fmt.Errorf("reactor telemetry reconcile interval must be positive and no greater than one minute")
	}
	if strings.TrimSpace(c.IngestBaseURL) == "" {
		return fmt.Errorf("reactor telemetry ingest base URL is required")
	}
	if strings.TrimSpace(c.CredentialSigningKey) == "" {
		return fmt.Errorf("reactor telemetry credential signing key is required")
	}
	return nil
}

type RegisterDynamicReactorRequest struct {
	GameSessionID  string `json:"gameSessionId"`
	ReactorID      string `json:"reactorId"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type RemoveDynamicReactorRequest struct {
	GameSessionID  string `json:"gameSessionId"`
	ReactorID      string `json:"reactorId"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type ReactorTelemetryGatewayProfile struct {
	IngestBaseURL string `json:"ingestBaseUrl"`
	IngestToken   string `json:"-"`
}

type ReactorTelemetryWorker struct {
	WorkerID          string                         `json:"workerId"`
	SourceID          string                         `json:"sourceId"`
	GameSessionID     string                         `json:"gameSessionId"`
	ReactorID         string                         `json:"reactorId"`
	WorkerIndex       int                            `json:"workerIndex"`
	MaxFrames         uint64                         `json:"maxFrames"`
	Gateway           ReactorTelemetryGatewayProfile `json:"-"`
	BrokerURL         string                         `json:"-"`
	DatabaseURL       string                         `json:"-"`
	LakeURL           string                         `json:"-"`
	ContainerSocket   string                         `json:"-"`
	ClusterCredential string                         `json:"-"`
}

type ReactorTelemetryWorkerSet struct {
	SetID                string                    `json:"setId"`
	GameSessionID        string                    `json:"gameSessionId"`
	ReactorID            string                    `json:"reactorId"`
	RegisterIdempotency  string                    `json:"-"`
	RemoveIdempotency    string                    `json:"-"`
	Lifecycle            ReactorTelemetryLifecycle `json:"lifecycle"`
	Workers              []ReactorTelemetryWorker  `json:"workers"`
	CredentialsRevoked   bool                      `json:"credentialsRevoked"`
	CreatedAt            time.Time                 `json:"createdAt"`
	UpdatedAt            time.Time                 `json:"updatedAt"`
	ExpiresAt            time.Time                 `json:"expiresAt"`
	MeasuredRetentionSec int64                     `json:"measuredRetentionSec"`
	CleanupDeadline      time.Time                 `json:"cleanupDeadline,omitempty"`
	LastError            string                    `json:"lastError,omitempty"`
}

type ReactorTelemetryLaunch struct {
	SetID   string                   `json:"setId"`
	Workers []ReactorTelemetryWorker `json:"workers"`
}

type ReactorTelemetryRuntime interface {
	StartWorkerSet(ctx context.Context, launch ReactorTelemetryLaunch) error
	StopWorkerSet(ctx context.Context, setID string) error
}

type ReactorTelemetryStore interface {
	GetWorkerSet(gameSessionID, reactorID string) (ReactorTelemetryWorkerSet, error)
	FindRegistration(gameSessionID, idempotencyKey string) (ReactorTelemetryWorkerSet, error)
	ListWorkerSets(gameSessionID string) ([]ReactorTelemetryWorkerSet, error)
	SaveWorkerSet(set ReactorTelemetryWorkerSet) error
}

type ReactorTelemetrySourceRegistrar interface {
	RegisterSource(source ScadaResidentSourceDeclaration) (int, error)
}

type ReactorTelemetryManager struct {
	cfg        ReactorTelemetryConfig
	store      ReactorTelemetryStore
	runtime    ReactorTelemetryRuntime
	sources    ReactorTelemetrySourceRegistrar
	now        func() time.Time
	transition sync.Mutex
}

func NewReactorTelemetryManager(cfg ReactorTelemetryConfig, store ReactorTelemetryStore, runtime ReactorTelemetryRuntime, sources ReactorTelemetrySourceRegistrar) *ReactorTelemetryManager {
	if store == nil {
		store = NewInMemoryReactorTelemetryStore()
	}
	if runtime == nil {
		runtime = ContractReactorTelemetryRuntime{}
	}
	return &ReactorTelemetryManager{cfg: cfg, store: store, runtime: runtime, sources: sources, now: time.Now}
}

func (m *ReactorTelemetryManager) TouchSession(gameSessionID string) error {
	m.transition.Lock()
	defer m.transition.Unlock()
	sets, err := m.store.ListWorkerSets(gameSessionID)
	if err != nil {
		return err
	}
	now := m.now().UTC()
	for _, set := range sets {
		if set.Lifecycle == ReactorTelemetryRemoved || set.Lifecycle == ReactorTelemetryCleanup || set.Lifecycle == ReactorTelemetryCleanupFailed {
			continue
		}
		set.UpdatedAt = now
		set.ExpiresAt = now.Add(m.cfg.SessionTTL)
		if err := m.store.SaveWorkerSet(set); err != nil {
			return err
		}
	}
	return nil
}

func (m *ReactorTelemetryManager) RegisterDynamicReactor(ctx context.Context, request RegisterDynamicReactorRequest) (ReactorTelemetryWorkerSet, bool, error) {
	m.transition.Lock()
	defer m.transition.Unlock()
	if err := m.cfg.Validate(); err != nil {
		return ReactorTelemetryWorkerSet{}, false, err
	}
	if err := validateDynamicReactorIdentity(request.GameSessionID, request.ReactorID, request.IdempotencyKey); err != nil {
		return ReactorTelemetryWorkerSet{}, false, err
	}
	if existing, err := m.store.FindRegistration(request.GameSessionID, request.IdempotencyKey); err == nil {
		if existing.Lifecycle == ReactorTelemetryStarting || existing.Lifecycle == ReactorTelemetryLaunchFailed {
			recovered, retryErr := m.retryWorkerSet(ctx, existing)
			return recovered, false, retryErr
		}
		return existing, false, nil
	} else if !errors.Is(err, ErrReactorTelemetryNotFound) {
		return ReactorTelemetryWorkerSet{}, false, err
	}
	if existing, err := m.store.GetWorkerSet(request.GameSessionID, request.ReactorID); err == nil {
		if existing.Lifecycle == ReactorTelemetryStarting || existing.Lifecycle == ReactorTelemetryLaunchFailed {
			recovered, retryErr := m.retryWorkerSet(ctx, existing)
			return recovered, false, retryErr
		}
		return existing, false, nil
	} else if err != nil && !errors.Is(err, ErrReactorTelemetryNotFound) {
		return ReactorTelemetryWorkerSet{}, false, err
	}
	sets, err := m.store.ListWorkerSets(request.GameSessionID)
	if err != nil {
		return ReactorTelemetryWorkerSet{}, false, err
	}
	active := 0
	for _, set := range sets {
		if set.Lifecycle != ReactorTelemetryRemoved {
			active++
		}
	}
	if active >= m.cfg.MaxReactorsPerSession {
		return ReactorTelemetryWorkerSet{}, false, ErrReactorTelemetrySessionCap
	}

	now := m.now().UTC()
	set := ReactorTelemetryWorkerSet{
		SetID:                stableTelemetryID("rts", request.GameSessionID, request.ReactorID),
		GameSessionID:        request.GameSessionID,
		ReactorID:            request.ReactorID,
		RegisterIdempotency:  request.IdempotencyKey,
		Lifecycle:            ReactorTelemetryStarting,
		CreatedAt:            now,
		UpdatedAt:            now,
		ExpiresAt:            now.Add(m.cfg.SessionTTL),
		MeasuredRetentionSec: int64(m.cfg.MeasuredRetention / time.Second),
	}
	for index := 0; index < m.cfg.WorkersPerSet; index++ {
		sourceID := stableTelemetryID("src", request.GameSessionID, request.ReactorID, fmt.Sprintf("%d", index+1))
		worker := ReactorTelemetryWorker{
			WorkerID:      stableTelemetryID("rtw", request.GameSessionID, request.ReactorID, fmt.Sprintf("%d", index+1)),
			SourceID:      sourceID,
			GameSessionID: request.GameSessionID,
			ReactorID:     request.ReactorID,
			WorkerIndex:   index,
			MaxFrames:     uint64((m.cfg.SessionTTL + time.Second - 1) / time.Second),
			Gateway: ReactorTelemetryGatewayProfile{
				IngestBaseURL: strings.TrimRight(m.cfg.IngestBaseURL, "/"),
				IngestToken:   m.issueSourceCredential(set.SetID, sourceID, request.ReactorID),
			},
		}
		set.Workers = append(set.Workers, worker)
		if m.sources != nil {
			if _, err := m.sources.RegisterSource(BuildReactorResidentSource(worker)); err != nil {
				return ReactorTelemetryWorkerSet{}, false, fmt.Errorf("register resident source %s: %w", sourceID, err)
			}
		}
	}
	if err := m.store.SaveWorkerSet(set); err != nil {
		return ReactorTelemetryWorkerSet{}, false, err
	}
	if err := m.runtime.StartWorkerSet(ctx, ReactorTelemetryLaunch{SetID: set.SetID, Workers: cloneTelemetryWorkers(set.Workers)}); err != nil {
		set.Lifecycle = ReactorTelemetryLaunchFailed
		set.LastError = err.Error()
		set.UpdatedAt = m.now().UTC()
		_ = m.store.SaveWorkerSet(set)
		return set, true, fmt.Errorf("start reactor telemetry worker set: %w", err)
	}
	set.Lifecycle = ReactorTelemetryActive
	set.UpdatedAt = m.now().UTC()
	if err := m.store.SaveWorkerSet(set); err != nil {
		return set, true, err
	}
	return set, true, nil
}

func (m *ReactorTelemetryManager) retryWorkerSet(ctx context.Context, set ReactorTelemetryWorkerSet) (ReactorTelemetryWorkerSet, error) {
	set.Lifecycle = ReactorTelemetryStarting
	set.LastError = ""
	set.UpdatedAt = m.now().UTC()
	for index := range set.Workers {
		worker := &set.Workers[index]
		worker.Gateway.IngestToken = m.issueSourceCredential(set.SetID, worker.SourceID, set.ReactorID)
		if m.sources != nil {
			if _, err := m.sources.RegisterSource(BuildReactorResidentSource(*worker)); err != nil {
				return set, fmt.Errorf("register resident source %s: %w", worker.SourceID, err)
			}
		}
	}
	if err := m.store.SaveWorkerSet(set); err != nil {
		return set, err
	}
	if err := m.runtime.StartWorkerSet(ctx, ReactorTelemetryLaunch{SetID: set.SetID, Workers: cloneTelemetryWorkers(set.Workers)}); err != nil {
		set.Lifecycle = ReactorTelemetryLaunchFailed
		set.LastError = err.Error()
		set.UpdatedAt = m.now().UTC()
		_ = m.store.SaveWorkerSet(set)
		return set, fmt.Errorf("start reactor telemetry worker set: %w", err)
	}
	set.Lifecycle = ReactorTelemetryActive
	set.UpdatedAt = m.now().UTC()
	if err := m.store.SaveWorkerSet(set); err != nil {
		return set, err
	}
	return set, nil
}

func (m *ReactorTelemetryManager) RemoveDynamicReactor(ctx context.Context, request RemoveDynamicReactorRequest) (ReactorTelemetryWorkerSet, error) {
	m.transition.Lock()
	defer m.transition.Unlock()
	if err := validateDynamicReactorIdentity(request.GameSessionID, request.ReactorID, request.IdempotencyKey); err != nil {
		return ReactorTelemetryWorkerSet{}, err
	}
	set, err := m.store.GetWorkerSet(request.GameSessionID, request.ReactorID)
	if err != nil {
		return ReactorTelemetryWorkerSet{}, err
	}
	if set.Lifecycle == ReactorTelemetryRemoved {
		return set, nil
	}
	return m.cleanupWorkerSet(ctx, set, request.IdempotencyKey)
}

func (m *ReactorTelemetryManager) cleanupWorkerSet(ctx context.Context, set ReactorTelemetryWorkerSet, idempotencyKey string) (ReactorTelemetryWorkerSet, error) {
	now := m.now().UTC()
	if idempotencyKey != "" {
		set.RemoveIdempotency = idempotencyKey
	}
	set.CredentialsRevoked = true
	set.Lifecycle = ReactorTelemetryCleanup
	if set.CleanupDeadline.IsZero() {
		set.CleanupDeadline = now.Add(m.cfg.CleanupTimeout)
	}
	set.UpdatedAt = now
	set.LastError = ""
	if err := m.store.SaveWorkerSet(set); err != nil {
		return set, err
	}
	cleanupContext, cancel := context.WithTimeout(ctx, m.cfg.CleanupTimeout)
	defer cancel()
	if err := m.runtime.StopWorkerSet(cleanupContext, set.SetID); err != nil {
		set.Lifecycle = ReactorTelemetryCleanupFailed
		set.LastError = err.Error()
		set.UpdatedAt = m.now().UTC()
		_ = m.store.SaveWorkerSet(set)
		return set, fmt.Errorf("stop reactor telemetry worker set: %w", err)
	}
	set.Lifecycle = ReactorTelemetryRemoved
	set.UpdatedAt = m.now().UTC()
	if err := m.store.SaveWorkerSet(set); err != nil {
		return set, err
	}
	return set, nil
}

func (m *ReactorTelemetryManager) AuthorizeSourceCredential(token, sourceID, reactorID string) bool {
	claims, ok := m.verifySourceCredential(token)
	if !ok || claims.SourceID != sourceID || claims.ReactorID != reactorID {
		return false
	}
	sets, err := m.store.ListWorkerSets("")
	if err != nil {
		return false
	}
	for _, set := range sets {
		lifecycleAuthorized := set.Lifecycle == ReactorTelemetryStarting || set.Lifecycle == ReactorTelemetryActive
		if set.SetID != claims.SetID || set.ReactorID != reactorID || set.CredentialsRevoked || !lifecycleAuthorized || !m.now().UTC().Before(set.ExpiresAt) {
			continue
		}
		for _, worker := range set.Workers {
			if worker.SourceID == sourceID {
				return true
			}
		}
	}
	return false
}

func (m *ReactorTelemetryManager) ReconcileExpired(ctx context.Context) error {
	m.transition.Lock()
	defer m.transition.Unlock()
	sets, err := m.store.ListWorkerSets("")
	if err != nil {
		return err
	}
	var reconcileErr error
	now := m.now().UTC()
	for _, set := range sets {
		if set.Lifecycle == ReactorTelemetryRemoved {
			continue
		}
		cleanupPending := set.Lifecycle == ReactorTelemetryCleanup || set.Lifecycle == ReactorTelemetryCleanupFailed
		if !cleanupPending && now.Before(set.ExpiresAt) {
			continue
		}
		idempotencyKey := set.RemoveIdempotency
		if idempotencyKey == "" {
			idempotencyKey = "expire-" + set.SetID
		}
		_, err := m.cleanupWorkerSet(ctx, set, idempotencyKey)
		if err != nil {
			reconcileErr = errors.Join(reconcileErr, err)
		}
	}
	return reconcileErr
}

func (m *ReactorTelemetryManager) ReconcileActive(ctx context.Context) error {
	m.transition.Lock()
	defer m.transition.Unlock()
	sets, err := m.store.ListWorkerSets("")
	if err != nil {
		return err
	}
	var reconcileErr error
	for _, set := range sets {
		if set.Lifecycle == ReactorTelemetryCleanup || set.Lifecycle == ReactorTelemetryCleanupFailed {
			if _, err := m.cleanupWorkerSet(ctx, set, set.RemoveIdempotency); err != nil {
				reconcileErr = errors.Join(reconcileErr, err)
			}
			continue
		}
		if set.Lifecycle != ReactorTelemetryActive && set.Lifecycle != ReactorTelemetryStarting && set.Lifecycle != ReactorTelemetryLaunchFailed {
			continue
		}
		if !m.now().UTC().Before(set.ExpiresAt) {
			continue
		}
		if _, err := m.retryWorkerSet(ctx, set); err != nil {
			reconcileErr = errors.Join(reconcileErr, err)
		}
	}
	return reconcileErr
}

type sourceCredentialClaims struct {
	SetID     string `json:"setId"`
	SourceID  string `json:"sourceId"`
	ReactorID string `json:"reactorId"`
}

func (m *ReactorTelemetryManager) issueSourceCredential(setID, sourceID, reactorID string) string {
	raw, _ := json.Marshal(sourceCredentialClaims{SetID: setID, SourceID: sourceID, ReactorID: reactorID})
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(m.cfg.CredentialSigningKey))
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *ReactorTelemetryManager) verifySourceCredential(token string) (sourceCredentialClaims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return sourceCredentialClaims{}, false
	}
	mac := hmac.New(sha256.New, []byte(m.cfg.CredentialSigningKey))
	_, _ = mac.Write([]byte(parts[0]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(got, want) {
		return sourceCredentialClaims{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return sourceCredentialClaims{}, false
	}
	var claims sourceCredentialClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return sourceCredentialClaims{}, false
	}
	return claims, claims.SetID != "" && claims.SourceID != "" && claims.ReactorID != ""
}

type ContractReactorTelemetryRuntime struct{}

func (ContractReactorTelemetryRuntime) StartWorkerSet(context.Context, ReactorTelemetryLaunch) error {
	return nil
}
func (ContractReactorTelemetryRuntime) StopWorkerSet(context.Context, string) error { return nil }

type InMemoryReactorTelemetryStore struct {
	mu   sync.RWMutex
	sets map[string]ReactorTelemetryWorkerSet
}

func NewInMemoryReactorTelemetryStore() *InMemoryReactorTelemetryStore {
	return &InMemoryReactorTelemetryStore{sets: make(map[string]ReactorTelemetryWorkerSet)}
}

func (s *InMemoryReactorTelemetryStore) GetWorkerSet(gameSessionID, reactorID string) (ReactorTelemetryWorkerSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, ok := s.sets[telemetryStoreKey(gameSessionID, reactorID)]
	if !ok {
		return ReactorTelemetryWorkerSet{}, ErrReactorTelemetryNotFound
	}
	return cloneTelemetrySet(set), nil
}

func (s *InMemoryReactorTelemetryStore) FindRegistration(gameSessionID, idempotencyKey string) (ReactorTelemetryWorkerSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, set := range s.sets {
		if set.GameSessionID == gameSessionID && set.RegisterIdempotency == idempotencyKey {
			return cloneTelemetrySet(set), nil
		}
	}
	return ReactorTelemetryWorkerSet{}, ErrReactorTelemetryNotFound
}

func (s *InMemoryReactorTelemetryStore) ListWorkerSets(gameSessionID string) ([]ReactorTelemetryWorkerSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sets := make([]ReactorTelemetryWorkerSet, 0, len(s.sets))
	for _, set := range s.sets {
		if gameSessionID == "" || set.GameSessionID == gameSessionID {
			sets = append(sets, cloneTelemetrySet(set))
		}
	}
	return sets, nil
}

func (s *InMemoryReactorTelemetryStore) SaveWorkerSet(set ReactorTelemetryWorkerSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets[telemetryStoreKey(set.GameSessionID, set.ReactorID)] = redactTelemetryCredentials(set)
	return nil
}

func BuildReactorResidentSource(worker ReactorTelemetryWorker) ScadaResidentSourceDeclaration {
	tags := reactorTelemetryTags(worker)
	return ScadaResidentSourceDeclaration{
		SchemaVersion: WorkbenchSourceSchemaVersion, SourceID: worker.SourceID, ReactorID: worker.ReactorID,
		DisplayName: "Reactor-scoped public-safe resident source stand-in", Lifecycle: "resident",
		SyntheticStatus: WorkbenchSyntheticPublicStandin,
		Ingest:          ScadaIngest{Topic: "scada.telemetry.v1", EndpointKind: "gateway-http"}, Tags: tags,
	}
}

func BuildReactorTelemetryFrames(worker ReactorTelemetryWorker, sequence uint64, sampledAt time.Time) []ScadaTelemetryFrame {
	frames := make([]ScadaTelemetryFrame, 0, 2)
	for _, tag := range reactorTelemetryTags(worker) {
		frames = append(frames, ScadaTelemetryFrame{
			SchemaVersion: WorkbenchScadaSchemaVersion, SourceID: worker.SourceID, ReactorID: worker.ReactorID,
			TagID: tag.TagID, AssetID: tag.AssetID, SignalKind: tag.SignalKind,
			SampledAt: sampledAt.UTC(), ObservedAt: sampledAt.UTC().Add(time.Second), Sequence: sequence,
			Unit: tag.Unit, Value: reactorTelemetryValue(tag.SignalKind, sequence), Quality: "good",
			ValueBasis: WorkbenchValueMeasured, SyntheticStatus: WorkbenchSyntheticPublicStandin,
		})
	}
	return frames
}

func reactorTelemetryTags(worker ReactorTelemetryWorker) []ScadaSourceTag {
	prefix := worker.SourceID
	asset := stableTelemetryID("asset", worker.ReactorID)
	groups := [][]ScadaSourceTag{
		{{TagID: prefix + "-flux", AssetID: asset, SignalKind: ScadaSignalFlux, Unit: "relative-flux", ValueBasis: WorkbenchValueMeasured}, {TagID: prefix + "-temperature", AssetID: asset, SignalKind: ScadaSignalTemperature, Unit: "degC", ValueBasis: WorkbenchValueMeasured}},
		{{TagID: prefix + "-pressure", AssetID: asset, SignalKind: ScadaSignalPressure, Unit: "MPa", ValueBasis: WorkbenchValueMeasured}, {TagID: prefix + "-actuator", AssetID: asset, SignalKind: ScadaSignalActuatorState, Unit: "state", ValueBasis: WorkbenchValueMeasured}},
		{{TagID: prefix + "-electrical", AssetID: asset, SignalKind: ScadaSignalElectricalState, Unit: "state", ValueBasis: WorkbenchValueMeasured}, {TagID: prefix + "-comms", AssetID: asset, SignalKind: ScadaSignalComms, Unit: "ms", ValueBasis: WorkbenchValueMeasured}},
	}
	index := worker.WorkerIndex
	if index < 0 || index >= len(groups) {
		index = 0
	}
	for i := range groups[index] {
		groups[index][i].SourceID = worker.SourceID
		groups[index][i].ReactorID = worker.ReactorID
	}
	return groups[index]
}

func reactorTelemetryValue(kind ScadaSignalKind, sequence uint64) map[string]any {
	step := float64(sequence - 1)
	switch kind {
	case ScadaSignalFlux:
		return map[string]any{"scalar": 0.82 + step*0.002}
	case ScadaSignalTemperature:
		return map[string]any{"scalar": 612.4 + step*0.4}
	case ScadaSignalPressure:
		return map[string]any{"scalar": 15.2 + step*0.02}
	case ScadaSignalActuatorState:
		return map[string]any{"state": "position-hold", "positionPct": 63}
	case ScadaSignalElectricalState:
		return map[string]any{"voltageKv": 13.8, "breakerClosed": true}
	default:
		return map[string]any{"latencyMs": 18.4 + step*0.3, "packetLossPct": 0.2}
	}
}

func stableTelemetryID(prefix string, parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + "-" + hex.EncodeToString(hash[:8])
}

func validateDynamicReactorIdentity(sessionID, reactorID, idempotencyKey string) error {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(reactorID) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return fmt.Errorf("gameSessionId, reactorId, and idempotencyKey are required")
	}
	return nil
}

func telemetryStoreKey(sessionID, reactorID string) string { return sessionID + "\x00" + reactorID }

func cloneTelemetrySet(set ReactorTelemetryWorkerSet) ReactorTelemetryWorkerSet {
	set.Workers = cloneTelemetryWorkers(set.Workers)
	return set
}

func cloneTelemetryWorkers(workers []ReactorTelemetryWorker) []ReactorTelemetryWorker {
	return append([]ReactorTelemetryWorker(nil), workers...)
}

func redactTelemetryCredentials(set ReactorTelemetryWorkerSet) ReactorTelemetryWorkerSet {
	set = cloneTelemetrySet(set)
	for index := range set.Workers {
		set.Workers[index].Gateway.IngestToken = ""
	}
	return set
}
