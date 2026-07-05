package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSimopsArtifactWriterExternalModeCanResolveConfiguredBinary(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig().Simops
	cfg.IcebergWriterMode = "external"
	cfg.IcebergRustCommand = visibleIcebergRustCommandForTest(t)
	cfg.IcebergManifestDir = t.TempDir()

	if _, err := NewSimopsArtifactWriter(cfg, NewInMemorySimopsStore(), time.Now); err != nil {
		t.Fatalf("expected external writer to initialize with visible command %q, got: %v", cfg.IcebergRustCommand, err)
	}
}

func visibleIcebergRustCommandForTest(t *testing.T) string {
	t.Helper()

	candidates := []string{
		strings.TrimSpace(os.Getenv("SIMOPS_ICEBERG_RUST_CMD")),
		"iceberg-playground",
		"iceberg",
		"/usr/local/bin/iceberg-playground",
	}

	if gopath := strings.TrimSpace(os.Getenv("GOPATH")); gopath != "" {
		candidates = append(candidates, filepath.Join(gopath, "bin", "iceberg"))
	}
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		candidates = append(candidates, filepath.Join(home, "go", "bin", "iceberg"))
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		fields := strings.Fields(candidate)
		if len(fields) == 0 {
			continue
		}
		if _, err := exec.LookPath(fields[0]); err == nil {
			return candidate
		} else {
			t.Logf("iceberg command probe failed: %q (%v)", fields[0], err)
		}
	}

	t.Fatalf("no visible iceberg binary found. set SIMOPS_ICEBERG_RUST_CMD to an install path or put the binary on PATH")
	return ""
}

func TestSimopsArtifactWriterRejectsExternalWithoutCommand(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig().Simops
	cfg.IcebergWriterMode = "external"
	cfg.IcebergRustCommand = ""
	cfg.IcebergManifestDir = t.TempDir()

	if _, err := NewSimopsArtifactWriter(cfg, NewInMemorySimopsStore(), time.Now); err == nil {
		t.Fatalf("expected external writer without command to fail")
	}
}

func TestSimopsArtifactWriterManifestTransitionsAndManifest(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig().Simops
	cfg.IcebergWriterMode = "manifest"
	cfg.IcebergManifestDir = t.TempDir()
	runID := "RUN-STUB-WRITER"
	run := SimopsRunRecord{
		RunID:       runID,
		Source:      "frontend",
		Lifecycle:   SimopsCreated,
		CreatedAt:   time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
		SubmittedBy: "writer-test",
	}
	store := NewInMemorySimopsStore()
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}

	planner := IcebergArtifactPlanner{Warehouse: "s3://radiant-simops/warehouse", Bucket: "radiant-simops", Catalog: "postgres-sql"}
	artifact := planner.PlanArtifact(run)
	if err := store.SaveArtifact(artifact); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}

	writer, err := NewSimopsArtifactWriter(cfg, store, time.Now)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	prepared, err := writer.Prepare(SimopsArtifactWritePlan{
		Artifact:   artifact,
		Topic:      cfg.RedpandaTopic,
		Sequence:   4,
		EventCount: 11,
	})
	if err != nil {
		t.Fatalf("prepare artifact: %v", err)
	}
	if prepared.Artifact.Status != SimopsArtifactStatusPrepared {
		t.Fatalf("expected prepared artifact status, got %q", prepared.Artifact.Status)
	}
	if prepared.Artifact.Location == "" {
		t.Fatalf("expected manifest location")
	}
	if err := writer.WriteArtifact(runID, prepared); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writer.Commit(runID); err != nil {
		t.Fatalf("commit artifact: %v", err)
	}

	artifacts, err := store.ListArtifacts(runID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one artifact, got %d", len(artifacts))
	}
	if artifacts[0].Status != SimopsArtifactStatusCommitted {
		t.Fatalf("expected committed status, got %q", artifacts[0].Status)
	}

	data, err := os.ReadFile(filepath.Clean(prepared.Artifact.Location))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var intent SimopsArtifactCommitIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		t.Fatalf("decode manifest payload: %v", err)
	}
	if intent.RunID != runID {
		t.Fatalf("expected manifest run_id %q, got %q", runID, intent.RunID)
	}
	if intent.Sequence != 4 || intent.EventCount != 11 {
		t.Fatalf("unexpected manifest payload %#v", intent)
	}
}

func TestSimopsArtifactIntentProcessorPublishesIntentReadyEvents(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig().Simops
	cfg.IcebergWriterMode = "manifest"
	cfg.IcebergManifestDir = t.TempDir()
	runID := "RUN-INTENT-PROCESS"
	run := SimopsRunRecord{
		RunID:       runID,
		Source:      "frontend",
		Lifecycle:   SimopsCreated,
		CreatedAt:   time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
		SubmittedBy: "writer-test",
	}
	store := NewInMemorySimopsStore()
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}

	planner := IcebergArtifactPlanner{Warehouse: "s3://radiant-simops/warehouse", Bucket: "radiant-simops", Catalog: "postgres-sql"}
	artifact := planner.PlanArtifact(run)
	if err := store.SaveArtifact(artifact); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}

	writer, err := NewSimopsArtifactWriter(cfg, store, time.Now)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	capturingEventLog := &capturingSimopsEventLog{}
	processor := NewSimopsArtifactIntentProcessor(writer, capturingEventLog, cfg.RedpandaTopic, 2, func() time.Time { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) })

	if _, err := processor.ProcessEvents(context.Background(),
		SimopsEvent{RunID: runID, EventType: "worker.telemetry"},
		SimopsEvent{RunID: runID, EventType: "worker.telemetry"},
	); err != nil {
		t.Fatalf("process events: %v", err)
	}

	events := capturingEventLog.List()
	if len(events) != 1 {
		t.Fatalf("expected one artifact intent event, got %d", len(events))
	}
	if events[0].EventType != SimopsArtifactIntentEventType {
		t.Fatalf("expected event type %q, got %q", SimopsArtifactIntentEventType, events[0].EventType)
	}
	var intent SimopsArtifactCommitIntent
	if err := json.Unmarshal(events[0].Frame, &intent); err != nil {
		t.Fatalf("decode artifact intent: %v", err)
	}
	if intent.RunID != runID {
		t.Fatalf("expected run id %q, got %q", runID, intent.RunID)
	}
	if intent.EventCount != 2 {
		t.Fatalf("expected two-events intent, got %d", intent.EventCount)
	}
	if intent.Sequence != 1 {
		t.Fatalf("expected sequence 1 for first intent, got %d", intent.Sequence)
	}
	if !strings.Contains(intent.Location, filepath.Join(cfg.IcebergManifestDir, runID)) {
		t.Fatalf("expected manifest path under run directory, got %q", intent.Location)
	}

	artifacts, err := store.ListArtifacts(runID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if artifacts[0].Status != SimopsArtifactStatusCommitted {
		t.Fatalf("expected committed status, got %q", artifacts[0].Status)
	}
}

func TestSimopsArtifactWriterExternalModeFailsWhenCommandUnavailable(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig().Simops
	cfg.IcebergWriterMode = "external"
	cfg.IcebergRustCommand = "nonexistent-iceberg-rust-binary"
	cfg.IcebergManifestDir = t.TempDir()

	if _, err := NewSimopsArtifactWriter(cfg, NewInMemorySimopsStore(), time.Now); err == nil || !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("expected unavailable command failure, got %v", err)
	}
}

type capturingSimopsEventLog struct {
	mu     sync.Mutex
	events []SimopsEvent
}

func (l *capturingSimopsEventLog) Publish(_ context.Context, event SimopsEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
	return nil
}

func (l *capturingSimopsEventLog) List() []SimopsEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]SimopsEvent(nil), l.events...)
}
