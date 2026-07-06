package gateway

import (
	"encoding/json"
	"time"
)

type SimopsLifecycle string

const (
	SimopsCreated   SimopsLifecycle = "created"
	SimopsStarting  SimopsLifecycle = "starting"
	SimopsStreaming SimopsLifecycle = "streaming"
	SimopsDegraded  SimopsLifecycle = "degraded"
	SimopsComplete  SimopsLifecycle = "complete"
	SimopsFailed    SimopsLifecycle = "failed"
	SimopsStopped   SimopsLifecycle = "stopped"
)

type SimopsWorkerKind string

const (
	SimopsWorkerScheduler SimopsWorkerKind = "scheduler"
	SimopsWorkerStorage   SimopsWorkerKind = "storage"
	SimopsWorkerBurst     SimopsWorkerKind = "burst"
	SimopsWorkerFabric    SimopsWorkerKind = "fabric"
)

var defaultSimopsWorkers = []SimopsWorkerKind{
	SimopsWorkerScheduler,
	SimopsWorkerStorage,
	SimopsWorkerBurst,
	SimopsWorkerFabric,
}

type SimopsRunRequest struct {
	ScenarioID      string   `json:"scenario_id"`
	Source          string   `json:"source,omitempty"`
	WorkScript      string   `json:"work_script,omitempty"`
	LaunchMode      string   `json:"launch_mode,omitempty"`
	WorkerKinds     []string `json:"worker_kinds,omitempty"`
	RuntimeLimitSec int      `json:"runtime_limit_sec,omitempty"`
	IdempotencyKey  string   `json:"idempotency_key,omitempty"`
}

type SimopsRunResponse struct {
	RunID           string                 `json:"run_id"`
	ScenarioID      string                 `json:"scenario_id"`
	Lifecycle       SimopsLifecycle        `json:"lifecycle"`
	Source          string                 `json:"source"`
	LaunchMode      string                 `json:"launch_mode"`
	RuntimeLimitSec int                    `json:"runtime_limit_sec"`
	Created         bool                   `json:"created"`
	SubmittedBy     string                 `json:"submitted_by"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	MoQSubscription SimopsMoQSubscription  `json:"moq_subscription"`
	Workers         []SimopsWorkerRecord   `json:"workers"`
	SpoolCommands   []SimopsSpoolCommand   `json:"spool_commands"`
	Artifacts       []SimopsArtifactRecord `json:"artifacts"`
}

type SimopsRunRecord struct {
	RunID           string
	ScenarioID      string
	Lifecycle       SimopsLifecycle
	Source          string
	WorkScript      string
	LaunchMode      string
	RuntimeLimitSec int
	IdempotencyKey  string
	SubmittedBy     string
	IngestToken     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SimopsWorkerRecord struct {
	RunID      string            `json:"-"`
	WorkerID   string            `json:"worker_id"`
	WorkerKind SimopsWorkerKind  `json:"worker_kind"`
	Lifecycle  SimopsLifecycle   `json:"lifecycle"`
	LaunchMode string            `json:"launch_mode"`
	Endpoint   string            `json:"endpoint,omitempty"`
	Frames     int               `json:"frames"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type SimopsSpoolCommand struct {
	CommandID string          `json:"command_id"`
	RunID     string          `json:"run_id"`
	WorkerID  string          `json:"worker_id"`
	Mode      string          `json:"mode"`
	State     SimopsLifecycle `json:"state"`
	Message   string          `json:"message"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type SimopsArtifactRecord struct {
	ArtifactID   string    `json:"artifact_id"`
	RunID        string    `json:"run_id"`
	Kind         string    `json:"kind"`
	MediaType    string    `json:"media_type"`
	Location     string    `json:"location"`
	IcebergTable string    `json:"iceberg_table,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type SimopsMoQSubscription struct {
	Protocol  string           `json:"protocol"`
	Endpoint  string           `json:"endpoint"`
	Namespace string           `json:"namespace"`
	Token     string           `json:"token"`
	ExpiresAt time.Time        `json:"expires_at"`
	Tracks    []SimopsMoQTrack `json:"tracks"`
}

type SimopsMoQTrack struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	WorkerID   string `json:"worker_id,omitempty"`
	WorkerKind string `json:"worker_kind,omitempty"`
}

type SimopsTelemetryFrame struct {
	SchemaVersion string           `json:"schemaVersion"`
	RunID         string           `json:"runId"`
	ScenarioID    string           `json:"scenarioId"`
	WorkerID      string           `json:"workerId"`
	WorkerKind    SimopsWorkerKind `json:"workerKind"`
	Sequence      uint64           `json:"sequence"`
	EmittedAt     string           `json:"emittedAt"`
	ReceivedAt    string           `json:"receivedAt,omitempty"`
	PayloadType   string           `json:"payloadType"`
	StreamQuality json.RawMessage  `json:"streamQuality,omitempty"`
	Payload       json.RawMessage  `json:"payload"`
}

type SimopsTelemetryBatch struct {
	Frames []SimopsTelemetryFrame `json:"frames"`
}

type SimopsEvent struct {
	RunID             string          `json:"run_id"`
	WorkerID          string          `json:"worker_id,omitempty"`
	EventType         string          `json:"event_type"`
	Lifecycle         SimopsLifecycle `json:"lifecycle,omitempty"`
	Frame             json.RawMessage `json:"frame,omitempty"`
	OccurredAt        time.Time       `json:"occurred_at"`
	RedpandaTopic     string          `json:"-"`
	RedpandaPartition int             `json:"-"`
	RedpandaOffset    int64           `json:"-"`
}
