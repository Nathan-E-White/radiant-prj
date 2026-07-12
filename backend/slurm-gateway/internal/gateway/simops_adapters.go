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
	StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error)
	StopRun(ctx context.Context, runID string) error
	SyncRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error)
}

type RunConnectionProfileSpooler interface {
	StartRunProfiles(ctx context.Context, run SimopsRunRecord, profiles []RunConnectionProfile) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error)
}

type RunConnectionProfileStopper interface {
	StopRunProfiles(ctx context.Context, runID string, profiles []RunConnectionProfile) error
}

type SimopsEventLog interface {
	Publish(ctx context.Context, event SimopsEvent) error
}

type SimopsArtifactSink interface {
	PlanArtifact(run SimopsRunRecord) SimopsArtifactRecord
}

type SimopsRuntime interface {
	StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error)
	StopRun(ctx context.Context, runID string) error
	SyncRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error)
}

type ContractSimopsSpooler struct {
	Mode string
	Now  func() time.Time
}

func (s ContractSimopsSpooler) StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	mode := strings.TrimSpace(s.Mode)
	if mode == "" {
		mode = "resident"
	}

	records := make([]SimopsWorkerRecord, 0, len(workers))
	commands := make([]SimopsSpoolCommand, 0, len(workers))
	for _, worker := range workers {
		workerID := fmt.Sprintf("%s-01", worker)
		records = append(records, SimopsWorkerRecord{
			RunID:      run.RunID,
			WorkerID:   workerID,
			WorkerKind: worker,
			Lifecycle:  SimopsStarting,
			LaunchMode: mode,
			Endpoint:   fmt.Sprintf("http://simops-bucket-%s:8080", worker),
			UpdatedAt:  now,
			Labels: map[string]string{
				"simops.redpanda.topic": "simops.telemetry.v1",
				"simops.mode":           mode,
			},
		})
		commands = append(commands, SimopsSpoolCommand{
			CommandID: fmt.Sprintf("%s-%s-start", run.RunID, worker),
			RunID:     run.RunID,
			WorkerID:  workerID,
			Mode:      mode,
			State:     SimopsStarting,
			Message:   "Bucket launch command accepted by contract spooler",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	return records, commands, nil
}

func (s ContractSimopsSpooler) StopRun(ctx context.Context, runID string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s ContractSimopsSpooler) SyncRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	observations := make([]ObservedWorkerLifecycle, 0, len(workers))
	for _, worker := range workers {
		observations = append(observations, ObservedWorkerLifecycle{
			RunID:      run.RunID,
			WorkerID:   worker.WorkerID,
			WorkerKind: worker.WorkerKind,
			State:      ObservedWorkerActive,
			Runtime:    "contract",
			Reason:     "contract-runtime",
			Message:    "contract runtime reports worker record present",
			ObservedAt: now,
			Labels:     worker.Labels,
		})
	}
	return observations, nil
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
