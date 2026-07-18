package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ArtifactForgeSchedulerDriftRecipe = "scheduler-drift"
	ArtifactForgeEligibleArtifactKind = "simulated-result-state"
	ArtifactForgeEligibleMediaType    = "application/vnd.radiant.simops-result+json"
	ArtifactForgeSimulatedMarginCard  = "artifactForge.simulated-margin-card.v1"
	ArtifactForgeArtifactCommitted    = "projection-committed"
	ArtifactForgeIntegrityVerified    = "verified"
)

var ErrArtifactForgeNotFound = errors.New("Artifact Forge request not found")
var ErrArtifactForgeResultArtifactNotFound = errors.New("Artifact Forge result artifact not found")

type ArtifactForgeDecision string

const (
	ArtifactForgeAwaitingRun         ArtifactForgeDecision = "awaiting-run"
	ArtifactForgeRunFailed           ArtifactForgeDecision = "run-failed"
	ArtifactForgeRunLaunchRetryable  ArtifactForgeDecision = "run-launch-retryable"
	ArtifactForgeTelemetryIneligible ArtifactForgeDecision = "operational-telemetry-ineligible"
	ArtifactForgeArtifactIncomplete  ArtifactForgeDecision = "artifact-incomplete"
	ArtifactForgeArtifactIneligible  ArtifactForgeDecision = "artifact-ineligible"
	ArtifactForgeIntegrityFailed     ArtifactForgeDecision = "artifact-integrity-failed"
	ArtifactForgeResultMissing       ArtifactForgeDecision = "simulated-result-missing"
	ArtifactForgeLineageMissing      ArtifactForgeDecision = "lineage-missing"
	ArtifactForgeLineageIneligible   ArtifactForgeDecision = "lineage-ineligible"
	ArtifactForgeOutcomeApplied      ArtifactForgeDecision = "outcome-applied"
	ArtifactForgeIntentRejected      ArtifactForgeDecision = "intent-rejected"
)

type ArtifactForgeEventType string

const (
	ArtifactForgeEventIntentAccepted       ArtifactForgeEventType = "artifact-forge.intent.accepted"
	ArtifactForgeEventIntentRejected       ArtifactForgeEventType = "artifact-forge.intent.rejected"
	ArtifactForgeEventRunAssociated        ArtifactForgeEventType = "artifact-forge.run.associated"
	ArtifactForgeEventRunLaunchRetryable   ArtifactForgeEventType = "artifact-forge.run.launch-retryable"
	ArtifactForgeEventEligibilityEvaluated ArtifactForgeEventType = "artifact-forge.eligibility.evaluated"
	ArtifactForgeEventOutcomeApplied       ArtifactForgeEventType = "artifact-forge.outcome.applied"
)

type ArtifactForgeRequest struct {
	GameSessionID      string `json:"gameSessionId"`
	ReactorID          string `json:"reactorId"`
	SimulationJobID    string `json:"simulationJobId"`
	SimulationJobState string `json:"simulationJobState"`
	SimulationRecipe   string `json:"simulationRecipe"`
	IdempotencyKey     string `json:"idempotencyKey"`
}

type ArtifactForgeGameOutcome struct {
	OutcomeID  string `json:"outcomeId"`
	Type       string `json:"type"`
	Version    int    `json:"version"`
	ReactorID  string `json:"reactorId"`
	ArtifactID string `json:"artifactId"`
	LineageID  string `json:"lineageId"`
	ValueID    string `json:"valueId"`
}

type ArtifactForgeResultArtifact struct {
	ArtifactID    string    `json:"artifactId"`
	RunID         string    `json:"runId"`
	Kind          string    `json:"kind"`
	MediaType     string    `json:"mediaType"`
	SchemaVersion string    `json:"schemaVersion"`
	Recipe        string    `json:"recipe"`
	Status        string    `json:"status"`
	Integrity     string    `json:"integrity"`
	Complete      bool      `json:"complete"`
	ExpectedValue string    `json:"expectedValue"`
	PersistedAt   time.Time `json:"persistedAt"`
}

type artifactForgeResultArtifactReader interface {
	ArtifactForgeResultArtifact(runID string) (ArtifactForgeResultArtifact, error)
}

type ArtifactForgeTrace struct {
	SimulationJobID string `json:"simulationJobId"`
	RunID           string `json:"runId,omitempty"`
	ArtifactID      string `json:"artifactId,omitempty"`
	LineageID       string `json:"lineageId,omitempty"`
}

type ArtifactForgeEvent struct {
	EventID         string                 `json:"eventId"`
	Type            ArtifactForgeEventType `json:"type"`
	GameSessionID   string                 `json:"gameSessionId"`
	ReactorID       string                 `json:"reactorId"`
	SimulationJobID string                 `json:"simulationJobId"`
	RunID           string                 `json:"runId,omitempty"`
	ArtifactID      string                 `json:"artifactId,omitempty"`
	LineageID       string                 `json:"lineageId,omitempty"`
	Decision        ArtifactForgeDecision  `json:"decision,omitempty"`
	OccurredAt      time.Time              `json:"occurredAt"`
}

type ArtifactForgeRecord struct {
	RequestID        string                    `json:"requestId"`
	GameSessionID    string                    `json:"gameSessionId"`
	ReactorID        string                    `json:"reactorId"`
	SimulationJobID  string                    `json:"simulationJobId"`
	SimulationRecipe string                    `json:"simulationRecipe"`
	IdempotencyKey   string                    `json:"idempotencyKey"`
	SubmittedBy      string                    `json:"submittedBy"`
	RunID            string                    `json:"runId,omitempty"`
	Decision         ArtifactForgeDecision     `json:"decision"`
	Message          string                    `json:"message"`
	Outcome          *ArtifactForgeGameOutcome `json:"outcome,omitempty"`
	Trace            ArtifactForgeTrace        `json:"trace"`
	Events           []ArtifactForgeEvent      `json:"events"`
	CreatedAt        time.Time                 `json:"createdAt"`
	UpdatedAt        time.Time                 `json:"updatedAt"`
	LastActivityAt   time.Time                 `json:"lastActivityAt"`
	SessionExpiresAt time.Time                 `json:"sessionExpiresAt"`
	RetainUntil      time.Time                 `json:"retainUntil"`
}

type ArtifactForgeStore interface {
	Find(gameSessionID, idempotencyKey string) (ArtifactForgeRecord, error)
	FindRun(runID string) (ArtifactForgeRecord, error)
	List(gameSessionID string) ([]ArtifactForgeRecord, error)
	Save(record ArtifactForgeRecord) error
	TouchSession(gameSessionID string, activityAt time.Time) error
	PruneExpired(now time.Time) (int64, error)
}

type InMemoryArtifactForgeStore struct {
	mu      sync.RWMutex
	records map[string]ArtifactForgeRecord
}

func NewInMemoryArtifactForgeStore() *InMemoryArtifactForgeStore {
	return &InMemoryArtifactForgeStore{records: make(map[string]ArtifactForgeRecord)}
}

func (s *InMemoryArtifactForgeStore) Find(gameSessionID, idempotencyKey string) (ArtifactForgeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[artifactForgeStoreKey(gameSessionID, idempotencyKey)]
	if !ok {
		return ArtifactForgeRecord{}, ErrArtifactForgeNotFound
	}
	return cloneArtifactForgeRecord(record), nil
}

func (s *InMemoryArtifactForgeStore) List(gameSessionID string) ([]ArtifactForgeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := []ArtifactForgeRecord{}
	for _, record := range s.records {
		if record.GameSessionID == gameSessionID {
			records = append(records, cloneArtifactForgeRecord(record))
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].RequestID < records[j].RequestID })
	return records, nil
}

func (s *InMemoryArtifactForgeStore) FindRun(runID string) (ArtifactForgeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, record := range s.records {
		if record.RunID == runID {
			return cloneArtifactForgeRecord(record), nil
		}
	}
	return ArtifactForgeRecord{}, ErrArtifactForgeNotFound
}

func (s *InMemoryArtifactForgeStore) Save(record ArtifactForgeRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := artifactForgeStoreKey(record.GameSessionID, record.IdempotencyKey)
	if existing, ok := s.records[key]; ok && existing.Outcome != nil {
		if record.Outcome == nil || *existing.Outcome != *record.Outcome {
			return fmt.Errorf("Artifact Forge applied outcome cannot be removed or replaced")
		}
	}
	s.records[key] = cloneArtifactForgeRecord(record)
	return nil
}

func (s *InMemoryArtifactForgeStore) TouchSession(gameSessionID string, activityAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, record := range s.records {
		if record.GameSessionID != gameSessionID {
			continue
		}
		setArtifactForgeRetention(&record, activityAt)
		s.records[key] = record
	}
	return nil
}

func (s *InMemoryArtifactForgeStore) PruneExpired(now time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed int64
	for key, record := range s.records {
		if !record.RetainUntil.IsZero() && !record.RetainUntil.After(now) {
			delete(s.records, key)
			removed++
		}
	}
	return removed, nil
}

type ArtifactForgeManager struct {
	mu        sync.Mutex
	store     ArtifactForgeStore
	simops    *SimopsController
	workbench WorkbenchStore
	now       func() time.Time
}

func NewArtifactForgeManager(store ArtifactForgeStore, simops *SimopsController, workbench WorkbenchStore) *ArtifactForgeManager {
	if store == nil {
		store = NewInMemoryArtifactForgeStore()
	}
	return &ArtifactForgeManager{store: store, simops: simops, workbench: workbench, now: time.Now}
}

func (m *ArtifactForgeManager) ResultLineageContext(runID string) ([]TwinLineageInput, []TwinLineageArtifact, bool) {
	record, err := m.store.FindRun(runID)
	if err != nil || record.Decision == ArtifactForgeIntentRejected {
		return nil, nil, false
	}
	inputs := []TwinLineageInput{
		{SourceKind: "game-session", SourceID: record.GameSessionID, ValueBasis: WorkbenchValueSimulated},
		{SourceKind: "fleet-reactor", SourceID: record.ReactorID, ValueBasis: WorkbenchValueSimulated},
		{SourceKind: "simulation-recipe", SourceID: record.SimulationRecipe, ValueBasis: WorkbenchValueSimulated},
	}
	artifactID := "simulated-result-" + runID
	artifacts := []TwinLineageArtifact{{ArtifactID: artifactID, Path: "simops://results/run_id=" + runID, MediaType: ArtifactForgeEligibleMediaType}}
	return inputs, artifacts, true
}

func (m *ArtifactForgeManager) TouchSession(gameSessionID string) error {
	return m.store.TouchSession(gameSessionID, m.now().UTC())
}

func (m *ArtifactForgeManager) Request(ctx context.Context, request ArtifactForgeRequest, identity string) (ArtifactForgeRecord, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalizeArtifactForgeRequest(&request)
	if err := validateArtifactForgeIdentity(request); err != nil {
		return ArtifactForgeRecord{}, false, err
	}
	if err := m.TouchSession(request.GameSessionID); err != nil {
		return ArtifactForgeRecord{}, false, err
	}

	existing, err := m.store.Find(request.GameSessionID, request.IdempotencyKey)
	if err == nil {
		if existing.ReactorID != request.ReactorID || existing.SimulationJobID != request.SimulationJobID || existing.SimulationRecipe != request.SimulationRecipe {
			return ArtifactForgeRecord{}, false, fmt.Errorf("idempotency key is already associated with a different Artifact Forge request")
		}
		if existing.Outcome != nil || existing.Decision == ArtifactForgeIntentRejected {
			return existing, false, nil
		}
		if existing.RunID == "" {
			return m.associateRun(ctx, existing, false)
		}
		return m.evaluate(existing)
	}
	if !errors.Is(err, ErrArtifactForgeNotFound) {
		return ArtifactForgeRecord{}, false, err
	}

	now := m.now().UTC()
	record := ArtifactForgeRecord{
		RequestID:        artifactForgeStableID("forge", request.GameSessionID, request.IdempotencyKey),
		GameSessionID:    request.GameSessionID,
		ReactorID:        request.ReactorID,
		SimulationJobID:  request.SimulationJobID,
		SimulationRecipe: request.SimulationRecipe,
		IdempotencyKey:   request.IdempotencyKey,
		SubmittedBy:      identity,
		Trace:            ArtifactForgeTrace{SimulationJobID: request.SimulationJobID},
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	setArtifactForgeRetention(&record, now)
	if request.SimulationJobState != "completed" {
		record.Decision = ArtifactForgeIntentRejected
		record.Message = "Artifact Forge requires one completed local Simulation Job; no SimOps Run was requested"
		record.Events = append(record.Events, m.event(record, ArtifactForgeEventIntentRejected))
		if err := m.store.Save(record); err != nil {
			return ArtifactForgeRecord{}, false, err
		}
		return record, true, nil
	}
	if request.SimulationRecipe != ArtifactForgeSchedulerDriftRecipe {
		record.Decision = ArtifactForgeIntentRejected
		record.Message = "simulation recipe is not on the Artifact Forge allowlist; no SimOps Run was requested"
		record.Events = append(record.Events, m.event(record, ArtifactForgeEventIntentRejected))
		if err := m.store.Save(record); err != nil {
			return ArtifactForgeRecord{}, false, err
		}
		return record, true, nil
	}
	if err := m.enforceCaps(record); err != nil {
		return ArtifactForgeRecord{}, false, err
	}

	record.Decision = ArtifactForgeAwaitingRun
	record.Message = "Artifact Forge intent accepted; the distinct SimOps Run has not reached a successful terminal state"
	record.Events = append(record.Events, m.event(record, ArtifactForgeEventIntentAccepted))
	if err := m.store.Save(record); err != nil {
		return ArtifactForgeRecord{}, false, err
	}
	return m.associateRun(ctx, record, true)
}

func (m *ArtifactForgeManager) ReconcileExpired() (int64, error) {
	return m.store.PruneExpired(m.now().UTC())
}

func (m *ArtifactForgeManager) associateRun(ctx context.Context, record ArtifactForgeRecord, created bool) (ArtifactForgeRecord, bool, error) {
	simopsIdentity := "artifact-forge:" + record.SubmittedBy + ":" + record.GameSessionID
	response, _, createErr := m.simops.CreateRun(ctx, SimopsRunRequest{
		ScenarioID: record.SimulationRecipe, Source: "frontend", WorkerKinds: []string{string(SimopsWorkerScheduler)}, IdempotencyKey: record.RequestID,
	}, simopsIdentity)
	if createErr != nil {
		recovered, lookupErr := m.simops.store.GetRunByIdempotency(simopsIdentity, record.RequestID)
		if lookupErr != nil {
			record.Decision = ArtifactForgeRunLaunchRetryable
			record.Message = "the SimOps Run launch did not complete; retry with the same idempotency key to recover the association"
			record.UpdatedAt = m.now().UTC()
			record.Events = appendEvaluationEvent(record.Events, m.event(record, ArtifactForgeEventRunLaunchRetryable))
			if err := m.store.Save(record); err != nil {
				return ArtifactForgeRecord{}, false, errors.Join(createErr, err)
			}
			return record, created, nil
		}
		response.RunID = recovered.RunID
	}
	record.RunID = response.RunID
	record.Trace.RunID = response.RunID
	record.Decision = ArtifactForgeAwaitingRun
	record.Message = "Artifact Forge intent accepted; the distinct SimOps Run has not reached a successful terminal state"
	record.UpdatedAt = m.now().UTC()
	record.Events = append(record.Events, m.event(record, ArtifactForgeEventRunAssociated))
	if err := m.store.Save(record); err != nil {
		return ArtifactForgeRecord{}, false, err
	}
	if createErr != nil {
		evaluated, _, err := m.evaluate(record)
		return evaluated, created, err
	}
	return record, created, nil
}

func (m *ArtifactForgeManager) evaluate(record ArtifactForgeRecord) (ArtifactForgeRecord, bool, error) {
	run, err := m.simops.store.GetRun(record.RunID)
	if err != nil {
		return ArtifactForgeRecord{}, false, err
	}
	switch run.Lifecycle {
	case SimopsFailed, SimopsStopped:
		return m.recordDecision(record, ArtifactForgeRunFailed, "the associated SimOps Run did not complete successfully; no game outcome was applied", ArtifactForgeTrace{})
	case SimopsComplete:
		// Continue through artifact, result, and Lineage eligibility.
	default:
		record.Decision = ArtifactForgeAwaitingRun
		record.Message = "the associated SimOps Run has not reached the successful terminal state; no game outcome was applied"
		record.UpdatedAt = m.now().UTC()
		if err := m.store.Save(record); err != nil {
			return ArtifactForgeRecord{}, false, err
		}
		return record, false, nil
	}

	artifactReader, ok := m.workbench.(artifactForgeResultArtifactReader)
	if !ok {
		return ArtifactForgeRecord{}, false, fmt.Errorf("Workbench store does not expose durable Artifact Forge result artifacts")
	}
	eligible, err := artifactReader.ArtifactForgeResultArtifact(record.RunID)
	if errors.Is(err, ErrArtifactForgeResultArtifactNotFound) {
		return m.recordDecision(record, ArtifactForgeTelemetryIneligible, "operational telemetry is ineligible for Artifact Forge rewards or evidence claims; no game outcome was applied", ArtifactForgeTrace{})
	}
	if err != nil {
		return ArtifactForgeRecord{}, false, err
	}
	trace := ArtifactForgeTrace{ArtifactID: eligible.ArtifactID}
	if eligible.Status != ArtifactForgeArtifactCommitted || !eligible.Complete {
		return m.recordDecision(record, ArtifactForgeArtifactIncomplete, "the simulated-result artifact is not durably committed and complete; no game outcome was applied", trace)
	}
	if eligible.Kind != ArtifactForgeEligibleArtifactKind || eligible.MediaType != ArtifactForgeEligibleMediaType || eligible.SchemaVersion != WorkbenchResultSchemaVersion || eligible.Recipe != record.SimulationRecipe {
		return m.recordDecision(record, ArtifactForgeArtifactIneligible, "the artifact kind, media type, schema version, or recipe is not allowlisted; no game outcome was applied", trace)
	}
	if eligible.Integrity != ArtifactForgeIntegrityVerified {
		return m.recordDecision(record, ArtifactForgeIntegrityFailed, "the simulated-result artifact failed integrity verification; no game outcome was applied", trace)
	}

	results, err := m.workbench.LatestResultFrames(100)
	if err != nil {
		return ArtifactForgeRecord{}, false, err
	}
	var result *SimopsResultFrame
	for index := range results {
		candidate := &results[index]
		if candidate.ScenarioID == record.SimulationRecipe && validateSimopsResultFrame(run, *candidate) == nil {
			result = candidate
			break
		}
	}
	if result == nil {
		return m.recordDecision(record, ArtifactForgeResultMissing, "the artifact has no eligible public-safe Simulated Result State; no game outcome was applied", trace)
	}
	expectedValue, ok := simopsResultValueByID(result.Values, eligible.ExpectedValue)
	if !ok {
		return m.recordDecision(record, ArtifactForgeResultMissing, "the artifact has no eligible expected Simulated Result State value; no game outcome was applied", trace)
	}
	lineage, err := m.workbench.LineageForValue(expectedValue.ValueID)
	if errors.Is(err, ErrWorkbenchNotFound) {
		return m.recordDecision(record, ArtifactForgeLineageMissing, "the Simulated Result State has no complete Lineage; no game outcome was applied", trace)
	}
	if err != nil {
		return ArtifactForgeRecord{}, false, err
	}
	if !artifactForgeLineageEligible(lineage, record, eligible) {
		return m.recordDecision(record, ArtifactForgeLineageIneligible, "Lineage does not bind the game session, reactor, Run, recipe, and artifact; no game outcome was applied", ArtifactForgeTrace{ArtifactID: eligible.ArtifactID, LineageID: lineage.LineageID})
	}

	record.Decision = ArtifactForgeOutcomeApplied
	record.Message = "one eligible simulated-result artifact produced the versioned Artifact Forge game outcome"
	record.Trace.ArtifactID = eligible.ArtifactID
	record.Trace.LineageID = lineage.LineageID
	record.Outcome = &ArtifactForgeGameOutcome{
		OutcomeID: artifactForgeStableID("outcome", record.RequestID, eligible.ArtifactID, lineage.LineageID), Type: ArtifactForgeSimulatedMarginCard, Version: 1,
		ReactorID: record.ReactorID, ArtifactID: eligible.ArtifactID, LineageID: lineage.LineageID, ValueID: expectedValue.ValueID,
	}
	record.UpdatedAt = m.now().UTC()
	record.Events = appendEvaluationEvent(record.Events, m.event(record, ArtifactForgeEventEligibilityEvaluated))
	record.Events = append(record.Events, m.event(record, ArtifactForgeEventOutcomeApplied))
	if err := m.store.Save(record); err != nil {
		return ArtifactForgeRecord{}, false, err
	}
	return record, false, nil
}

func (m *ArtifactForgeManager) recordDecision(record ArtifactForgeRecord, decision ArtifactForgeDecision, message string, trace ArtifactForgeTrace) (ArtifactForgeRecord, bool, error) {
	record.Decision = decision
	record.Message = message
	if trace.ArtifactID != "" {
		record.Trace.ArtifactID = trace.ArtifactID
	}
	if trace.LineageID != "" {
		record.Trace.LineageID = trace.LineageID
	}
	record.UpdatedAt = m.now().UTC()
	record.Events = appendEvaluationEvent(record.Events, m.event(record, ArtifactForgeEventEligibilityEvaluated))
	if err := m.store.Save(record); err != nil {
		return ArtifactForgeRecord{}, false, err
	}
	return record, false, nil
}

func (m *ArtifactForgeManager) enforceCaps(candidate ArtifactForgeRecord) error {
	records, err := m.store.List(candidate.GameSessionID)
	if err != nil {
		return err
	}
	active := 0
	for _, record := range records {
		if record.Outcome != nil || record.Decision == ArtifactForgeIntentRejected || record.Decision == ArtifactForgeRunFailed {
			continue
		}
		active++
		if record.ReactorID == candidate.ReactorID {
			return fmt.Errorf("reactor already has an active Artifact Forge request")
		}
	}
	if active >= 4 {
		return fmt.Errorf("game session already has four active Artifact Forge requests")
	}
	return nil
}

func (m *ArtifactForgeManager) event(record ArtifactForgeRecord, eventType ArtifactForgeEventType) ArtifactForgeEvent {
	index := len(record.Events) + 1
	return ArtifactForgeEvent{
		EventID: artifactForgeStableID("event", record.RequestID, fmt.Sprint(index), string(eventType)), Type: eventType,
		GameSessionID: record.GameSessionID, ReactorID: record.ReactorID, SimulationJobID: record.SimulationJobID,
		RunID: record.RunID, ArtifactID: record.Trace.ArtifactID, LineageID: record.Trace.LineageID, Decision: record.Decision, OccurredAt: m.now().UTC(),
	}
}

func artifactForgeLineageEligible(lineage DigitalTwinValueLineage, record ArtifactForgeRecord, artifact ArtifactForgeResultArtifact) bool {
	if lineage.SchemaVersion != WorkbenchLineageSchemaVersion || lineage.ValueBasis != WorkbenchValueSimulated || strings.TrimSpace(lineage.LineageID) == "" {
		return false
	}
	required := map[string]string{"game-session": record.GameSessionID, "fleet-reactor": record.ReactorID, "simulation-run": record.RunID, "simulation-recipe": record.SimulationRecipe}
	for kind, identity := range required {
		found := false
		for _, input := range lineage.Inputs {
			if input.SourceKind == kind && input.SourceID == identity {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, linked := range lineage.Artifacts {
		if linked.ArtifactID == artifact.ArtifactID && linked.MediaType == artifact.MediaType {
			return true
		}
	}
	return false
}

func buildArtifactForgeResultArtifact(frame SimopsResultFrame, persistedAt time.Time) ArtifactForgeResultArtifact {
	artifact := ArtifactForgeResultArtifact{
		ArtifactID: "simulated-result-" + frame.RunID, RunID: frame.RunID,
		Kind: ArtifactForgeEligibleArtifactKind, MediaType: ArtifactForgeEligibleMediaType,
		SchemaVersion: frame.SchemaVersion, Recipe: frame.ScenarioID, Status: ArtifactForgeArtifactCommitted,
		Integrity: "rejected", ExpectedValue: WorkbenchSimulatedMarginValue, PersistedAt: persistedAt.UTC(),
	}
	for _, value := range frame.Values {
		if value.ValueID == WorkbenchSimulatedMarginValue {
			artifact.Complete = true
		}
	}
	valid := frame.SchemaVersion == WorkbenchResultSchemaVersion && frame.ScenarioID == ArtifactForgeSchedulerDriftRecipe &&
		frame.ResultType == "syntheticEngineeringState" && frame.ValueBasis == WorkbenchValueSimulated &&
		frame.SyntheticStatus == WorkbenchSyntheticPublicStandin && strings.TrimSpace(frame.RunID) != "" &&
		strings.TrimSpace(frame.WorkerID) != "" && allowedWorker(frame.WorkerKind) && frame.Sequence > 0 &&
		strings.TrimSpace(frame.ProducedAt) != "" && strings.TrimSpace(frame.ModelID) != "" && artifact.Complete
	if valid {
		for _, value := range frame.Values {
			if strings.TrimSpace(value.ResultID) == "" || strings.TrimSpace(value.ValueID) == "" || len(value.Value) == 0 || string(value.Value) == "null" || value.Confidence < 0 || value.Confidence > 1 {
				valid = false
				break
			}
		}
	}
	if valid {
		artifact.Integrity = ArtifactForgeIntegrityVerified
	}
	return artifact
}

func normalizeArtifactForgeRequest(request *ArtifactForgeRequest) {
	request.GameSessionID = strings.TrimSpace(request.GameSessionID)
	request.ReactorID = strings.TrimSpace(request.ReactorID)
	request.SimulationJobID = strings.TrimSpace(request.SimulationJobID)
	request.SimulationJobState = strings.TrimSpace(request.SimulationJobState)
	request.SimulationRecipe = strings.TrimSpace(request.SimulationRecipe)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
}

func validateArtifactForgeIdentity(request ArtifactForgeRequest) error {
	for name, value := range map[string]string{"gameSessionId": request.GameSessionID, "reactorId": request.ReactorID, "simulationJobId": request.SimulationJobID, "idempotencyKey": request.IdempotencyKey} {
		if !runIDPattern.MatchString(value) {
			return fmt.Errorf("%s is required and must be an opaque stable identifier", name)
		}
	}
	if request.SimulationJobState == "" || request.SimulationRecipe == "" {
		return fmt.Errorf("simulationJobState and simulationRecipe are required")
	}
	return nil
}

func appendEvaluationEvent(events []ArtifactForgeEvent, event ArtifactForgeEvent) []ArtifactForgeEvent {
	if len(events) > 0 {
		last := events[len(events)-1]
		if last.Type == ArtifactForgeEventEligibilityEvaluated && last.Decision == event.Decision && last.ArtifactID == event.ArtifactID && last.LineageID == event.LineageID {
			return events
		}
	}
	return append(events, event)
}

func cloneArtifactForgeRecord(record ArtifactForgeRecord) ArtifactForgeRecord {
	record.Events = append([]ArtifactForgeEvent(nil), record.Events...)
	if record.Outcome != nil {
		outcome := *record.Outcome
		record.Outcome = &outcome
	}
	return record
}

func artifactForgeStoreKey(gameSessionID, idempotencyKey string) string {
	return gameSessionID + "\x00" + idempotencyKey
}

func artifactForgeStableID(prefix string, parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + "-" + hex.EncodeToString(digest[:8])
}

func setArtifactForgeRetention(record *ArtifactForgeRecord, activityAt time.Time) {
	record.LastActivityAt = activityAt.UTC()
	record.SessionExpiresAt = record.LastActivityAt.Add(24 * time.Hour)
	record.RetainUntil = record.SessionExpiresAt.Add(7 * 24 * time.Hour)
}
