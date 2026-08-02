package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type SimopsSpooler interface {
	StartRunProfiles(ctx context.Context, run SimopsRunRecord, profiles []RunConnectionProfile) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error)
	StopRunProfiles(ctx context.Context, runID string, profiles []RunConnectionProfile) error
	CleanupRunProfiles(ctx context.Context, runID string, profiles []RunConnectionProfile) error
	SyncRunProfiles(ctx context.Context, run SimopsRunRecord, profiles []RunConnectionProfile) ([]ObservedWorkerLifecycle, error)
}

type SimopsEventLog interface {
	Publish(ctx context.Context, event SimopsEvent) error
}

type SimopsArtifactSink interface {
	PlanArtifact(run SimopsRunRecord) SimopsArtifactRecord
}

type SimopsRuntime interface {
	StartRunProfiles(ctx context.Context, run SimopsRunRecord, profiles []RunConnectionProfile) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error)
	StopRunProfiles(ctx context.Context, runID string, profiles []RunConnectionProfile) error
	CleanupRunProfiles(ctx context.Context, runID string, profiles []RunConnectionProfile) error
	SyncRunProfiles(ctx context.Context, run SimopsRunRecord, profiles []RunConnectionProfile) ([]ObservedWorkerLifecycle, error)
}

type ContractSimopsSpooler struct {
	Mode string
	Now  func() time.Time
}

func (s ContractSimopsSpooler) StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	profiles := make([]RunConnectionProfile, 0, len(workers))
	mode := strings.TrimSpace(s.Mode)
	if mode == "" {
		mode = "resident"
	}
	for _, worker := range workers {
		workerID := fmt.Sprintf("%s-01", worker)
		profiles = append(profiles, RunConnectionProfile{
			RunID:      run.RunID,
			ScenarioID: run.ScenarioID,
			LaunchMode: mode,
			WorkerID:   workerID,
			WorkerKind: worker,
			Role:       RunConnectionRoleOrdinaryWorker,
			Gateway: RunGatewayConnection{
				IngestURL: fmt.Sprintf("http://simops-bucket-%s:8080", worker),
			},
			Labels: map[string]string{
				"simops.redpanda.topic": "simops.telemetry.v1",
				"simops.mode":           mode,
			},
		})
	}
	return s.StartRunProfiles(ctx, run, profiles)
}

func (s ContractSimopsSpooler) StartRunProfiles(ctx context.Context, run SimopsRunRecord, profiles []RunConnectionProfile) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	records := make([]SimopsWorkerRecord, 0, len(profiles))
	commands := make([]SimopsSpoolCommand, 0, len(profiles))
	for _, profile := range profiles {
		mode := strings.TrimSpace(profile.LaunchMode)
		if mode == "" {
			mode = strings.TrimSpace(s.Mode)
		}
		if mode == "" {
			mode = "resident"
		}
		runtimeID := contractRuntimeID(run.RunID, profile.WorkerID)
		labels := copyRunLabels(profile.Labels)
		if labels == nil {
			labels = map[string]string{}
		}
		labels["simops.runtime"] = "contract"
		labels["simops.runtime_adapter"] = "contract"
		records = append(records, SimopsWorkerRecord{
			RunID:      run.RunID,
			WorkerID:   profile.WorkerID,
			WorkerKind: profile.WorkerKind,
			Lifecycle:  SimopsStarting,
			LaunchMode: mode,
			Endpoint:   profile.Gateway.IngestURL,
			Runtime:    "contract",
			RuntimeID:  runtimeID,
			UpdatedAt:  now,
			Labels:     labels,
		})
		commands = append(commands, SimopsSpoolCommand{
			CommandID: fmt.Sprintf("%s-%s-start", run.RunID, profile.WorkerID),
			RunID:     run.RunID,
			WorkerID:  profile.WorkerID,
			Mode:      mode,
			State:     SimopsStarting,
			Message:   "Bucket launch command accepted by contract spooler",
			Metadata: map[string]string{
				"runtime_adapter": "contract",
				"runtime_id":      runtimeID,
				"requested_state": string(SimopsStarting),
				"worker_id":       profile.WorkerID,
				"worker_kind":     string(profile.WorkerKind),
			},
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	return records, commands, nil
}

func (s ContractSimopsSpooler) StopRun(ctx context.Context, runID string) error {
	return s.StopRunProfiles(ctx, runID, nil)
}

func (s ContractSimopsSpooler) StopRunProfiles(ctx context.Context, runID string, profiles []RunConnectionProfile) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s ContractSimopsSpooler) CleanupRunProfiles(ctx context.Context, runID string, profiles []RunConnectionProfile) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s ContractSimopsSpooler) SyncRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error) {
	profiles := make([]RunConnectionProfile, 0, len(workers))
	for _, worker := range workers {
		profiles = append(profiles, RunConnectionProfile{
			RunID:      run.RunID,
			WorkerID:   worker.WorkerID,
			WorkerKind: worker.WorkerKind,
			Labels:     worker.Labels,
		})
	}
	return s.SyncRunProfiles(ctx, run, profiles)
}

func (s ContractSimopsSpooler) SyncRunProfiles(ctx context.Context, run SimopsRunRecord, profiles []RunConnectionProfile) ([]ObservedWorkerLifecycle, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	observations := make([]ObservedWorkerLifecycle, 0, len(profiles))
	for _, profile := range profiles {
		state, reason, message := contractObservedLifecycle(run)
		observations = append(observations, ObservedWorkerLifecycle{
			RunID:      run.RunID,
			WorkerID:   profile.WorkerID,
			WorkerKind: profile.WorkerKind,
			State:      state,
			Runtime:    "contract",
			RuntimeID:  contractRuntimeID(run.RunID, profile.WorkerID),
			Reason:     reason,
			Message:    message,
			ObservedAt: now,
			Labels:     copyRunLabels(profile.Labels),
		})
	}
	return observations, nil
}

func contractObservedLifecycle(run SimopsRunRecord) (ObservedWorkerState, string, string) {
	switch run.Lifecycle {
	case SimopsComplete:
		return ObservedWorkerSucceeded, "contract-terminal", "contract runtime reports successful terminal worker state"
	case SimopsFailed, SimopsDegraded:
		return ObservedWorkerFailed, "contract-retryable-failure", "contract runtime reports a stable retryable worker failure"
	case SimopsStopped:
		return ObservedWorkerStopped, "contract-stopped", "contract runtime reports worker stopped by explicit request"
	default:
		return ObservedWorkerActive, "contract-runtime", "contract runtime reports worker record present"
	}
}

func contractRuntimeID(runID string, workerID string) string {
	return "contract://" + strings.TrimSpace(runID) + "/" + strings.TrimSpace(workerID)
}

func copyRunLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	copied := make(map[string]string, len(labels))
	for key, value := range labels {
		copied[key] = value
	}
	return copied
}

type MemorySimopsEventLog struct {
	Store SimopsStore
}

func (l MemorySimopsEventLog) Publish(ctx context.Context, event SimopsEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if l.Store == nil {
		return nil
	}
	return l.Store.SaveEvent(event)
}

func csvValues(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

type IcebergArtifactPlanner struct {
	Warehouse string
	Bucket    string
	Catalog   string
	Now       func() time.Time
}

func (p IcebergArtifactPlanner) PlanArtifact(run SimopsRunRecord) SimopsArtifactRecord {
	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}
	location := strings.TrimRight(strings.TrimSpace(p.Warehouse), "/")
	if location == "" {
		location = "s3://" + strings.TrimSpace(p.Bucket)
	}
	if location == "" {
		location = "s3://simops-warehouse"
	}
	if !strings.HasPrefix(location, "s3://") && !strings.HasPrefix(location, "file://") {
		location = "s3://" + strings.TrimPrefix(location, "file://")
	}
	location = strings.TrimRight(location, "/") + "/simops_telemetry/run_id=" + run.RunID
	if strings.TrimSpace(p.Catalog) == "" {
		location = strings.TrimRight(location, "/") + "/run=" + run.RunID
	}
	return SimopsArtifactRecord{
		ArtifactID:   "iceberg-telemetry-" + run.RunID,
		RunID:        run.RunID,
		Kind:         "iceberg-table-partition",
		MediaType:    "application/vnd.apache.iceberg.table",
		Status:       SimopsArtifactStatusReceived,
		Location:     location,
		IcebergTable: "simops.telemetry_frames",
		CreatedAt:    now,
	}
}

func buildMoQSubscription(cfg SimopsConfig, run SimopsRunRecord, workers []SimopsWorkerRecord, now time.Time) SimopsMoQSubscription {
	namespace := "radiant/simops/" + run.RunID
	tracks := []SimopsMoQTrack{
		{Name: "lifecycle", Role: "lifecycle"},
		{Name: "artifacts", Role: "artifacts"},
	}
	for _, worker := range workers {
		tracks = append(tracks,
			SimopsMoQTrack{
				Name:       "workers/" + worker.WorkerID + "/telemetry",
				Role:       "telemetry",
				WorkerID:   worker.WorkerID,
				WorkerKind: string(worker.WorkerKind),
			},
			SimopsMoQTrack{
				Name:       "workers/" + worker.WorkerID + "/quality",
				Role:       "quality",
				WorkerID:   worker.WorkerID,
				WorkerKind: string(worker.WorkerKind),
			},
		)
	}
	return SimopsMoQSubscription{
		Protocol:  "moq-webtransport",
		Endpoint:  cfg.MoQWebTransportURL,
		Namespace: namespace,
		Token:     randomToken(),
		ExpiresAt: now.Add(cfg.StreamTokenTTL).UTC(),
		Tracks:    tracks,
	}
}

func randomToken() string {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("simops-token-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}
