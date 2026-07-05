package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	SimopsArtifactStatusReceived  = "received"
	SimopsArtifactStatusPrepared  = "prepared"
	SimopsArtifactStatusCommitted = "committed"
	SimopsArtifactStatusFailed    = "failed"

	SimopsArtifactIntentEventType = "simops.artifact_ready"
)

const (
	simopsDefaultTopicFallback = "simops.telemetry.v1"
)

// SimopsArtifactWritePlan captures the normalized commit shape used by the
// Iceberg writer adapters.
type SimopsArtifactWritePlan struct {
	Artifact   SimopsArtifactRecord
	Topic      string
	Partition  string
	Sequence   uint64
	EventCount int
}

// SimopsArtifactCommitIntent is the payload emitted to both writer backends and
// event intents.
type SimopsArtifactCommitIntent struct {
	RunID      string `json:"run_id"`
	Sequence   uint64 `json:"sequence"`
	Topic      string `json:"topic"`
	EventCount int    `json:"event_count"`
	Partition  string `json:"partition"`
	ArtifactID string `json:"artifact_id"`
	Location   string `json:"location"`
}

// SimopsArtifactWriter defines the Iceberg writer contract.
type SimopsArtifactWriter interface {
	Prepare(plan SimopsArtifactWritePlan) (SimopsArtifactWritePlan, error)
	WriteArtifact(runID string, plan SimopsArtifactWritePlan) error
	Commit(runID string) error
}

// NewSimopsArtifactWriter constructs a writer adapter for the configured mode.
func NewSimopsArtifactWriter(cfg SimopsConfig, store SimopsStore, now func() time.Time) (SimopsArtifactWriter, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.IcebergWriterMode))
	if mode == "" {
		mode = "manifest"
	}

	manifestDir := strings.TrimSpace(cfg.IcebergManifestDir)
	if manifestDir == "" {
		manifestDir = "/tmp/simops-iceberg-manifests"
	}

	base := &simopsArtifactWriterBase{
		topic:       normalizeTopic(cfg.RedpandaTopic),
		manifestDir: strings.TrimRight(manifestDir, "/"),
		store:       store,
		now:         now,
	}

	switch mode {
	case "", "manifest", "stub":
		return &ManifestSimopsArtifactWriter{base: base}, nil
	case "disabled":
		return &DisabledSimopsArtifactWriter{base: base}, nil
	case "external":
		command := strings.Fields(strings.TrimSpace(cfg.IcebergRustCommand))
		if len(command) == 0 {
			return nil, fmt.Errorf("SIMOPS_ICEBERG_RUST_CMD is required when SIMOPS_ICEBERG_WRITER_MODE=external")
		}
		if err := ensureCommandAvailable(command); err != nil {
			return nil, err
		}
		return &ExternalCommandSimopsArtifactWriter{base: base, command: command}, nil
	default:
		return nil, fmt.Errorf("unsupported SIMOPS_ICEBERG_WRITER_MODE %q", cfg.IcebergWriterMode)
	}
}

// NewSimopsArtifactIntentProcessor creates a run-grouped artifact intent processor.
func NewSimopsArtifactIntentProcessor(writer SimopsArtifactWriter, eventLog SimopsEventLog, topic string, batchSize int, now func() time.Time) *SimopsArtifactIntentProcessor {
	if writer == nil {
		panic("SimopsArtifactIntentProcessor requires a writer")
	}
	if batchSize <= 0 {
		batchSize = 1
	}
	if now == nil {
		now = time.Now
	}
	return &SimopsArtifactIntentProcessor{
		writer:    writer,
		eventLog:  eventLog,
		topic:     normalizeTopic(topic),
		batchSize: batchSize,
		now:       now,
		states:    make(map[string]*simopsArtifactIntentRunState),
	}
}

func normalizeTopic(topic string) string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return simopsDefaultTopicFallback
	}
	return topic
}

type simopsArtifactWriterBase struct {
	store             SimopsStore
	topic             string
	manifestDir       string
	now               func() time.Time
	mu                sync.Mutex
	activeArtifactIDs map[string]string
}

func (w *simopsArtifactWriterBase) resolvedNow() time.Time {
	if w.now == nil {
		return time.Now().UTC()
	}
	return w.now().UTC()
}

func (w *simopsArtifactWriterBase) nextRunManifestPath(runID string, plan SimopsArtifactWritePlan) string {
	partition := strings.TrimSpace(plan.Partition)
	if partition == "" {
		partition = artifactPartition(runID)
	}
	filename := fmt.Sprintf("artifact-seq-%06d-events-%04d.json", plan.Sequence, plan.EventCount)
	return filepath.ToSlash(filepath.Join(
		strings.TrimSuffix(strings.TrimSpace(w.manifestDir), "/"),
		runID,
		partition,
		filename,
	))
}

func (w *simopsArtifactWriterBase) updateActiveArtifactID(runID string, artifactID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.activeArtifactIDs == nil {
		w.activeArtifactIDs = make(map[string]string)
	}
	w.activeArtifactIDs[runID] = artifactID
}

func (w *simopsArtifactWriterBase) activeArtifactID(runID string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.activeArtifactIDs == nil {
		return ""
	}
	return w.activeArtifactIDs[runID]
}

func (w *simopsArtifactWriterBase) ensureStoreStatus(runID string, artifactID string, status string) error {
	if w.store == nil || strings.TrimSpace(artifactID) == "" {
		return nil
	}
	if err := w.store.UpdateArtifactStatus(runID, artifactID, status); err != nil {
		return fmt.Errorf("update artifact status: %w", err)
	}
	return nil
}

func (w *simopsArtifactWriterBase) markRunFailed(runID string, err error, context string) error {
	artifactID := w.activeArtifactID(runID)
	if artifactID == "" {
		return fmt.Errorf("%s: %w", context, err)
	}
	if statusErr := w.ensureStoreStatus(runID, artifactID, SimopsArtifactStatusFailed); statusErr != nil {
		return statusErr
	}
	return fmt.Errorf("%s: %w", context, err)
}

func (w *simopsArtifactWriterBase) writeCommitPayload(plan SimopsArtifactWritePlan) ([]byte, error) {
	intent := SimopsArtifactCommitIntent{
		RunID:      strings.TrimSpace(plan.Artifact.RunID),
		Sequence:   plan.Sequence,
		Topic:      strings.TrimSpace(plan.Topic),
		EventCount: plan.EventCount,
		Partition:  strings.TrimSpace(plan.Partition),
		ArtifactID: strings.TrimSpace(plan.Artifact.ArtifactID),
		Location:   strings.TrimSpace(plan.Artifact.Location),
	}
	payload, err := json.Marshal(intent)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// ManifestSimopsArtifactWriter writes deterministic local manifest payloads and
// updates control-plane metadata via SimopsStore.
type ManifestSimopsArtifactWriter struct {
	base *simopsArtifactWriterBase
}

func (w *ManifestSimopsArtifactWriter) Prepare(plan SimopsArtifactWritePlan) (SimopsArtifactWritePlan, error) {
	plan.Topic = normalizeTopic(plan.Topic)
	if strings.TrimSpace(plan.Artifact.RunID) == "" {
		return plan, fmt.Errorf("artifact run_id is required")
	}
	if strings.TrimSpace(plan.Artifact.ArtifactID) == "" {
		return plan, fmt.Errorf("artifact_id is required")
	}
	plan.Artifact.Status = SimopsArtifactStatusPrepared
	plan.Partition = strings.TrimSpace(plan.Partition)
	if plan.Partition == "" {
		plan.Partition = artifactPartition(plan.Artifact.RunID)
	}
	plan.Artifact.Location = w.base.nextRunManifestPath(plan.Artifact.RunID, plan)
	w.base.updateActiveArtifactID(plan.Artifact.RunID, plan.Artifact.ArtifactID)
	if err := w.base.ensureStoreStatus(plan.Artifact.RunID, plan.Artifact.ArtifactID, SimopsArtifactStatusPrepared); err != nil {
		return plan, err
	}
	return plan, nil
}

func (w *ManifestSimopsArtifactWriter) WriteArtifact(runID string, plan SimopsArtifactWritePlan) error {
	plan.Topic = normalizeTopic(plan.Topic)
	payload, err := w.base.writeCommitPayload(plan)
	if err != nil {
		return w.base.markRunFailed(runID, err, "prepare manifest commit payload")
	}
	if strings.TrimSpace(plan.Artifact.Location) == "" {
		plan.Artifact.Location = w.base.nextRunManifestPath(runID, plan)
	}
	dir := filepath.Dir(plan.Artifact.Location)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return w.base.markRunFailed(runID, fmt.Errorf("create manifest directory %q: %w", dir, err), "prepare manifest commit destination")
	}
	if err := os.WriteFile(plan.Artifact.Location, payload, 0o600); err != nil {
		return w.base.markRunFailed(runID, fmt.Errorf("write artifact manifest %q: %w", plan.Artifact.Location, err), "write local artifact manifest")
	}
	if err := w.base.ensureStoreStatus(runID, plan.Artifact.ArtifactID, SimopsArtifactStatusPrepared); err != nil {
		return err
	}
	return nil
}

func (w *ManifestSimopsArtifactWriter) Commit(runID string) error {
	artifactID := w.base.activeArtifactID(runID)
	if strings.TrimSpace(artifactID) == "" {
		return fmt.Errorf("run %s has no prepared artifact", runID)
	}
	return w.base.ensureStoreStatus(runID, artifactID, SimopsArtifactStatusCommitted)
}

// ExternalCommandSimopsArtifactWriter runs an external process and forwards the
// normalized payload over stdin.
type ExternalCommandSimopsArtifactWriter struct {
	base    *simopsArtifactWriterBase
	command []string
}

func (w *ExternalCommandSimopsArtifactWriter) Prepare(plan SimopsArtifactWritePlan) (SimopsArtifactWritePlan, error) {
	plan.Topic = normalizeTopic(plan.Topic)
	if strings.TrimSpace(plan.Artifact.RunID) == "" {
		return plan, fmt.Errorf("artifact run_id is required")
	}
	if strings.TrimSpace(plan.Artifact.ArtifactID) == "" {
		return plan, fmt.Errorf("artifact_id is required")
	}
	plan.Artifact.Status = SimopsArtifactStatusPrepared
	plan.Partition = strings.TrimSpace(plan.Partition)
	if plan.Partition == "" {
		plan.Partition = artifactPartition(plan.Artifact.RunID)
	}
	plan.Artifact.Location = w.base.nextRunManifestPath(plan.Artifact.RunID, plan)
	w.base.updateActiveArtifactID(plan.Artifact.RunID, plan.Artifact.ArtifactID)
	if err := w.base.ensureStoreStatus(plan.Artifact.RunID, plan.Artifact.ArtifactID, SimopsArtifactStatusPrepared); err != nil {
		return plan, err
	}
	return plan, nil
}

func (w *ExternalCommandSimopsArtifactWriter) WriteArtifact(runID string, plan SimopsArtifactWritePlan) error {
	plan.Topic = normalizeTopic(plan.Topic)
	payload, err := w.base.writeCommitPayload(plan)
	if err != nil {
		return w.base.markRunFailed(runID, err, "prepare external commit payload")
	}
	if err := ensureCommandAvailable(w.command); err != nil {
		return w.base.markRunFailed(runID, err, "external iceberg command unavailable")
	}
	cmd := exec.CommandContext(context.Background(), w.command[0], w.command[1:]...)
	cmd.Stdin = bytes.NewBuffer(payload)
	combined, err := cmd.CombinedOutput()
	if err != nil {
		if strings.TrimSpace(string(combined)) != "" {
			return w.base.markRunFailed(runID, fmt.Errorf("iceberg command failed: %s", strings.TrimSpace(string(combined))), "run external iceberg command")
		}
		return w.base.markRunFailed(runID, fmt.Errorf("iceberg command failed: %w", err), "run external iceberg command")
	}
	if err := w.base.ensureStoreStatus(runID, plan.Artifact.ArtifactID, SimopsArtifactStatusPrepared); err != nil {
		return err
	}
	return nil
}

func (w *ExternalCommandSimopsArtifactWriter) Commit(runID string) error {
	artifactID := w.base.activeArtifactID(runID)
	if strings.TrimSpace(artifactID) == "" {
		return fmt.Errorf("run %s has no prepared artifact", runID)
	}
	if err := w.base.ensureStoreStatus(runID, artifactID, SimopsArtifactStatusCommitted); err != nil {
		return err
	}
	return nil
}

// DisabledSimopsArtifactWriter keeps the runtime path explicit while performing no-op
// writes.
type DisabledSimopsArtifactWriter struct {
	base *simopsArtifactWriterBase
}

func (w *DisabledSimopsArtifactWriter) Prepare(plan SimopsArtifactWritePlan) (SimopsArtifactWritePlan, error) {
	return plan, nil
}

func (w *DisabledSimopsArtifactWriter) WriteArtifact(runID string, plan SimopsArtifactWritePlan) error {
	return nil
}

func (w *DisabledSimopsArtifactWriter) Commit(runID string) error {
	return nil
}

func ensureCommandAvailable(command []string) error {
	if len(command) == 0 {
		return fmt.Errorf("empty command")
	}
	if command[0] == "" {
		return fmt.Errorf("empty command")
	}
	_, err := exec.LookPath(command[0])
	return err
}

func artifactPartition(runID string) string {
	return "run_id=" + strings.TrimSpace(runID)
}

type simopsArtifactIntentRunState struct {
	artifact  SimopsArtifactRecord
	sequence  uint64
	pending   int
	partition string
}

type SimopsArtifactIntentProcessor struct {
	writer    SimopsArtifactWriter
	eventLog  SimopsEventLog
	topic     string
	batchSize int
	now       func() time.Time

	mu     sync.Mutex
	states map[string]*simopsArtifactIntentRunState
}

func (p *SimopsArtifactIntentProcessor) ProcessEvents(ctx context.Context, events ...SimopsEvent) (int, error) {
	ready := 0
	for _, event := range events {
		count, err := p.ProcessEvent(ctx, event)
		if err != nil {
			return ready, err
		}
		ready += count
	}
	return ready, nil
}

func (p *SimopsArtifactIntentProcessor) ProcessEvent(ctx context.Context, event SimopsEvent) (int, error) {
	if strings.TrimSpace(event.RunID) == "" {
		return 0, nil
	}
	if event.EventType != "worker.telemetry" {
		return 0, nil
	}

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	p.mu.Lock()
	state := p.ensureRunState(event.RunID)
	state.pending++
	if state.pending < p.batchSize {
		p.mu.Unlock()
		return 0, nil
	}
	state.sequence++
	artifact := state.artifact
	sequence := state.sequence
	eventCount := state.pending
	partition := state.partition
	p.mu.Unlock()

	plan := SimopsArtifactWritePlan{
		Artifact:   artifact,
		Topic:      p.topic,
		Partition:  partition,
		Sequence:   sequence,
		EventCount: eventCount,
	}
	prepared, err := p.writer.Prepare(plan)
	if err != nil {
		return 0, err
	}
	if err := p.writer.WriteArtifact(event.RunID, prepared); err != nil {
		return 0, err
	}
	if err := p.writer.Commit(event.RunID); err != nil {
		return 0, err
	}

	p.mu.Lock()
	if state = p.states[event.RunID]; state != nil {
		state.pending = 0
		state.artifact = prepared.Artifact
	}
	p.mu.Unlock()

	intent := SimopsArtifactCommitIntent{
		RunID:      event.RunID,
		Sequence:   prepared.Sequence,
		Topic:      p.topic,
		EventCount: prepared.EventCount,
		Partition:  prepared.Partition,
		ArtifactID: prepared.Artifact.ArtifactID,
		Location:   prepared.Artifact.Location,
	}
	frame, err := json.Marshal(intent)
	if err != nil {
		return 0, err
	}
	if p.eventLog != nil {
		if err := p.eventLog.Publish(ctx, SimopsEvent{
			RunID:      event.RunID,
			EventType:  SimopsArtifactIntentEventType,
			Frame:      frame,
			OccurredAt: p.now(),
		}); err != nil {
			return 0, err
		}
	}

	return 1, nil
}

func (p *SimopsArtifactIntentProcessor) ensureRunState(runID string) *simopsArtifactIntentRunState {
	state, ok := p.states[runID]
	if !ok {
		artifactID := "iceberg-telemetry-" + runID
		state = &simopsArtifactIntentRunState{
			artifact: SimopsArtifactRecord{
				ArtifactID:   artifactID,
				RunID:        runID,
				Kind:         "iceberg-table-partition",
				MediaType:    "application/vnd.apache.iceberg.table",
				Status:       SimopsArtifactStatusReceived,
				Location:     "",
				IcebergTable: "simops.telemetry_frames",
			},
			partition: artifactPartition(runID),
		}
		p.states[runID] = state
	}
	return state
}
