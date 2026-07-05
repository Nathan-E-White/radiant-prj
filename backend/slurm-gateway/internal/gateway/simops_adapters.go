package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

type SimopsSpooler interface {
	StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error)
	StopRun(ctx context.Context, runID string) error
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

type DockerSimopsSpooler struct {
	Image         string
	ManifestRoot  string
	IngestBaseURL string
	FrameOverride int
	Network       string
	AutoRemove    bool
	LaunchMode    string
	CmdRunner     func(context.Context, ...string) (string, error)
}

func NewDockerSimopsSpooler(cfg SimopsConfig) DockerSimopsSpooler {
	runner := runDockerCommand
	return DockerSimopsSpooler{
		Image:         cfg.WorkerImage,
		ManifestRoot:  cfg.WorkerManifestRoot,
		IngestBaseURL: cfg.WorkerIngestBaseURL,
		FrameOverride: cfg.WorkerFrameOverride,
		Network:       cfg.WorkerNetwork,
		AutoRemove:    cfg.WorkerAutoRemove,
		LaunchMode:    cfg.LaunchMode,
		CmdRunner:     runner,
	}
}

func (s DockerSimopsSpooler) StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	if err := s.ensureImage(ctx); err != nil {
		return nil, nil, err
	}

	mode := strings.TrimSpace(s.LaunchMode)
	if mode == "" {
		mode = "resident"
	}

	records := make([]SimopsWorkerRecord, 0, len(workers))
	commands := make([]SimopsSpoolCommand, 0, len(workers))
	for _, worker := range workers {
		workerID := fmt.Sprintf("%s-01", worker)
		containerName := fmt.Sprintf("simops-%s-%s", run.RunID, workerID)
		manifest := path.Join(s.ManifestRoot, fmt.Sprintf("run-manifest.%s.json", run.ScenarioID))
		ingestURL := s.ingestURL(run.RunID)
		containerID, err := s.startWorker(ctx, run.RunID, containerName, workerID, worker, manifest, ingestURL, run.IngestToken)
		if err != nil {
			s.tryStopRunWorkers(ctx, run.RunID)
			return nil, nil, err
		}

		records = append(records, SimopsWorkerRecord{
			RunID:      run.RunID,
			WorkerID:   workerID,
			WorkerKind: worker,
			Lifecycle:  SimopsStarting,
			LaunchMode: mode,
			Endpoint:   ingestURL,
			UpdatedAt:  time.Now().UTC(),
			Labels: map[string]string{
				"simops.runtime":       "docker",
				"simops.worker_image":  s.Image,
				"simops.container_id":  containerID,
				"simops.worker_mode":   mode,
				"simops.launch_script": "simops-generator",
			},
		})
		commands = append(commands, SimopsSpoolCommand{
			CommandID: fmt.Sprintf("%s-%s-start", run.RunID, workerID),
			RunID:     run.RunID,
			WorkerID:  workerID,
			Mode:      mode,
			State:     SimopsStarting,
			Message:   fmt.Sprintf("Worker container launched as %s", containerName),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		})
	}

	return records, commands, nil
}

func (s DockerSimopsSpooler) StopRun(ctx context.Context, runID string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return s.tryStopRunWorkers(ctx, strings.TrimSpace(runID))
	}
}

func (s DockerSimopsSpooler) startWorker(ctx context.Context, runID, containerName, workerID string, worker SimopsWorkerKind, manifestPath string, ingestURL string, ingestToken string) (string, error) {
	args := []string{"run", "-d", "--name", containerName, "--label", "simops.run_id=" + runID, "--label", "simops.worker_id=" + workerID, "--label", "simops.worker_kind=" + string(worker)}
	if s.Network != "" {
		args = append(args, "--network", s.Network)
	}
	if s.AutoRemove {
		args = append(args, "--rm")
	}
	args = append(args, s.Image, "--manifest", manifestPath, "--worker", string(worker), "--run-id", runID, "--ingest-url", ingestURL, "--ingest-token", ingestToken, "--output", "-")
	if s.FrameOverride > 0 {
		args = append(args, "--frames", fmt.Sprintf("%d", s.FrameOverride))
	}

	containerID, err := s.CmdRunner(ctx, args...)
	if err != nil {
		return "", err
	}
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return "", fmt.Errorf("empty container id returned from docker run")
	}
	return containerID, nil
}

func (s DockerSimopsSpooler) ingestURL(runID string) string {
	base := strings.TrimRight(strings.TrimSpace(s.IngestBaseURL), "/")
	return base + "/internal/simops/runs/" + strings.TrimSpace(runID) + "/ingest"
}

func (s DockerSimopsSpooler) ensureImage(ctx context.Context) error {
	if strings.TrimSpace(s.Image) == "" {
		return fmt.Errorf("worker image is required")
	}
	output, err := s.CmdRunner(ctx, "image", "inspect", s.Image)
	if err == nil {
		if strings.TrimSpace(output) == "" {
			return fmt.Errorf("simops worker image %q inspect returned empty output", s.Image)
		}
		return nil
	}
	return fmt.Errorf("simops worker image %q not available locally", s.Image)
}

func (s DockerSimopsSpooler) tryStopRunWorkers(ctx context.Context, runID string) error {
	if runID == "" {
		return nil
	}
	ids, err := s.listRunContainers(ctx, runID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if stopErr := s.stopContainer(ctx, id); stopErr != nil {
			if err == nil {
				err = stopErr
			}
		}
	}
	return err
}

func (s DockerSimopsSpooler) listRunContainers(ctx context.Context, runID string) ([]string, error) {
	filter := "label=simops.run_id=" + runID
	output, err := s.CmdRunner(ctx, "ps", "-a", "--filter", filter, "--format", "{{.ID}}")
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	for _, candidate := range strings.Split(output, "\n") {
		containerID := strings.TrimSpace(candidate)
		if containerID == "" {
			continue
		}
		ids = append(ids, containerID)
	}
	return ids, nil
}

func (s DockerSimopsSpooler) stopContainer(ctx context.Context, containerID string) error {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return nil
	}
	_, _ = s.CmdRunner(ctx, "stop", "--time", "10", containerID)
	_, err := s.CmdRunner(ctx, "rm", "--force", containerID)
	return err
}

func runDockerCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
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

type kafkaMessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

type RedpandaEventLog struct {
	Topic  string
	Store  SimopsStore
	Writer kafkaMessageWriter
}

func NewRedpandaEventLog(cfg SimopsConfig, store SimopsStore) (*RedpandaEventLog, error) {
	brokers := csvValues(cfg.RedpandaBrokers)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("redpanda event log requires brokers")
	}
	if strings.TrimSpace(cfg.RedpandaTopic) == "" {
		return nil, fmt.Errorf("redpanda event log requires topic")
	}
	return &RedpandaEventLog{
		Topic: cfg.RedpandaTopic,
		Store: store,
		Writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        cfg.RedpandaTopic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
		},
	}, nil
}

func (l *RedpandaEventLog) Publish(ctx context.Context, event SimopsEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if l.Writer == nil {
		return fmt.Errorf("redpanda event log requires writer")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	key := event.RunID
	if event.WorkerID != "" {
		key += "|" + event.WorkerID
	}
	if err := l.Writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: payload,
		Time:  event.OccurredAt,
	}); err != nil {
		return err
	}
	if l.Store != nil {
		return l.Store.SaveEvent(event)
	}
	return nil
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
