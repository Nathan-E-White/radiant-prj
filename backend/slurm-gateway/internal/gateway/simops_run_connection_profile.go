package gateway

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type RunConnectionRole string

const (
	RunConnectionRoleOrdinaryWorker RunConnectionRole = "ordinary-worker"
	RunConnectionRoleStreamGateway  RunConnectionRole = "stream-gateway"
	RunConnectionRoleProjector      RunConnectionRole = "projector"
	RunConnectionRoleIcebergWriter  RunConnectionRole = "iceberg-writer"
)

type RunConnectionProfile struct {
	RunID        string
	ScenarioID   string
	LaunchMode   string
	WorkerID     string
	WorkerKind   SimopsWorkerKind
	Role         RunConnectionRole
	Gateway      RunGatewayConnection
	WorkerImage  string
	ManifestPath string
	Labels       map[string]string
	Runtime      RunRuntimeConnection
	Cleanup      RunCleanupPolicy
	DataPlane    *RunDataPlaneConnection
}

type RunGatewayConnection struct {
	IngestURL       string
	ResultIngestURL string
	IngestToken     string
}

type RunRuntimeConnection struct {
	Docker     DockerRunConnection
	Kubernetes KubernetesRunConnection
}

type DockerRunConnection struct {
	Network       string
	ContainerName string
	AutoRemove    bool
}

type KubernetesRunConnection struct {
	Namespace      string
	JobName        string
	ServiceAccount string
}

type RunCleanupPolicy struct {
	TTLSecondsAfterFinished int32
	AutoRemove              bool
}

type RunDataPlaneConnection struct {
	Redpanda RunRedpandaConnection
	Postgres RunPostgresConnection
	Iceberg  RunIcebergConnection
}

type RunRedpandaConnection struct {
	Brokers []string
	Topic   string
}

type RunPostgresConnection struct {
	DSN string
}

type RunIcebergConnection struct {
	Catalog       string
	CatalogDSN    string
	Warehouse     string
	S3Endpoint    string
	S3Bucket      string
	S3Region      string
	S3AccessKeyID string
	S3SecretKey   string
}

func BuildRunWorkerCommand(profile RunConnectionProfile, frameOverride int) []string {
	args := []string{
		"--manifest", profile.ManifestPath,
		"--worker", string(profile.WorkerKind),
		"--run-id", profile.RunID,
		"--ingest-url", profile.Gateway.IngestURL,
		"--ingest-token", profile.Gateway.IngestToken,
		"--result-ingest-url", profile.Gateway.ResultIngestURL,
		"--result-ingest-token", profile.Gateway.IngestToken,
		"--output", "-",
	}
	if frameOverride > 0 {
		args = append(args, "--frames", strconv.Itoa(frameOverride))
	}
	return args
}

func BuildRunWorkerConnectionProfiles(cfg SimopsConfig, run SimopsRunRecord, workers []SimopsWorkerKind) ([]RunConnectionProfile, error) {
	profiles := make([]RunConnectionProfile, 0, len(workers))
	for _, worker := range workers {
		profile, err := BuildRunWorkerConnectionProfile(cfg, run, worker)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func BuildRunWorkerConnectionProfile(cfg SimopsConfig, run SimopsRunRecord, worker SimopsWorkerKind) (RunConnectionProfile, error) {
	if !allowedWorker(worker) {
		return RunConnectionProfile{}, fmt.Errorf("worker kind %q is not supported", worker)
	}
	return buildRunConnectionProfile(cfg, run, RunConnectionRoleOrdinaryWorker, fmt.Sprintf("%s-01", worker), worker, false)
}

func BuildRunWorkerConnectionProfileForRecord(cfg SimopsConfig, run SimopsRunRecord, worker SimopsWorkerRecord) (RunConnectionProfile, error) {
	if !allowedWorker(worker.WorkerKind) {
		return RunConnectionProfile{}, fmt.Errorf("worker kind %q is not supported", worker.WorkerKind)
	}
	return buildRunConnectionProfile(cfg, run, RunConnectionRoleOrdinaryWorker, worker.WorkerID, worker.WorkerKind, false)
}

func BuildRunWorkerConnectionProfilesForRecords(cfg SimopsConfig, run SimopsRunRecord, workers []SimopsWorkerRecord) ([]RunConnectionProfile, error) {
	profiles := make([]RunConnectionProfile, 0, len(workers))
	for _, worker := range workers {
		profile, err := BuildRunWorkerConnectionProfileForRecord(cfg, run, worker)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func BuildTrustedRunConnectionProfile(cfg SimopsConfig, run SimopsRunRecord, role RunConnectionRole) (RunConnectionProfile, error) {
	switch role {
	case RunConnectionRoleStreamGateway, RunConnectionRoleProjector, RunConnectionRoleIcebergWriter:
	default:
		return RunConnectionProfile{}, fmt.Errorf("role %q is not trusted for data-plane refs", role)
	}
	return buildRunConnectionProfile(cfg, run, role, string(role), "", true)
}

func buildRunConnectionProfile(cfg SimopsConfig, run SimopsRunRecord, role RunConnectionRole, workerID string, worker SimopsWorkerKind, includeDataPlane bool) (RunConnectionProfile, error) {
	if err := validateRunProfileInputs(cfg, run); err != nil {
		return RunConnectionProfile{}, err
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return RunConnectionProfile{}, fmt.Errorf("worker identity is required")
	}

	runID := strings.TrimSpace(run.RunID)
	scenarioID := strings.TrimSpace(run.ScenarioID)
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.WorkerIngestBaseURL), "/")
	mode := strings.TrimSpace(run.LaunchMode)
	if mode == "" {
		mode = strings.TrimSpace(cfg.LaunchMode)
	}
	if mode == "" {
		mode = "resident"
	}

	profile := RunConnectionProfile{
		RunID:        runID,
		ScenarioID:   scenarioID,
		LaunchMode:   mode,
		WorkerID:     workerID,
		WorkerKind:   worker,
		Role:         role,
		WorkerImage:  strings.TrimSpace(cfg.WorkerImage),
		ManifestPath: path.Join(strings.TrimRight(strings.TrimSpace(cfg.WorkerManifestRoot), "/"), fmt.Sprintf("run-manifest.%s.json", scenarioID)),
		Gateway: RunGatewayConnection{
			IngestURL:       baseURL + "/internal/simops/runs/" + runID + "/ingest",
			ResultIngestURL: baseURL + "/internal/simops/runs/" + runID + "/results",
			IngestToken:     strings.TrimSpace(run.IngestToken),
		},
		Runtime: RunRuntimeConnection{
			Docker: DockerRunConnection{
				Network:       strings.TrimSpace(cfg.WorkerNetwork),
				ContainerName: dockerRunContainerName(runID, workerID),
				AutoRemove:    cfg.WorkerAutoRemove,
			},
			Kubernetes: KubernetesRunConnection{
				Namespace:      strings.TrimSpace(cfg.WorkerKubernetesNamespace),
				JobName:        kubernetesRunJobName(runID, workerID),
				ServiceAccount: strings.TrimSpace(cfg.WorkerKubernetesServiceAccount),
			},
		},
		Cleanup: RunCleanupPolicy{
			TTLSecondsAfterFinished: int32(cfg.WorkerCleanupTTL / time.Second),
			AutoRemove:              cfg.WorkerAutoRemove,
		},
	}
	profile.Labels = runConnectionLabels(profile)
	if includeDataPlane {
		profile.DataPlane = buildRunDataPlaneConnection(cfg)
	}
	return profile, nil
}

func validateRunProfileInputs(cfg SimopsConfig, run SimopsRunRecord) error {
	if strings.TrimSpace(run.RunID) == "" {
		return fmt.Errorf("run identity is required")
	}
	if strings.TrimSpace(run.ScenarioID) == "" {
		return fmt.Errorf("scenario identity is required")
	}
	if strings.TrimSpace(run.IngestToken) == "" {
		return fmt.Errorf("run ingest token is required")
	}
	if strings.TrimSpace(cfg.WorkerImage) == "" {
		return fmt.Errorf("SIMOPS_WORKER_IMAGE is required to build a run connection profile")
	}
	if strings.TrimSpace(cfg.WorkerManifestRoot) == "" {
		return fmt.Errorf("SIMOPS_WORKER_MANIFEST_ROOT is required to build a run connection profile")
	}
	if strings.TrimSpace(cfg.WorkerIngestBaseURL) == "" {
		return fmt.Errorf("SIMOPS_WORKER_INGEST_BASE_URL is required to build a run connection profile")
	}
	if strings.TrimSpace(cfg.WorkerKubernetesNamespace) == "" {
		return fmt.Errorf("SIMOPS_WORKER_KUBERNETES_NAMESPACE is required to build a run connection profile")
	}
	if cfg.WorkerCleanupTTL < 0 {
		return fmt.Errorf("SIMOPS_WORKER_CLEANUP_TTL must be zero or positive")
	}
	return nil
}

func runConnectionLabels(profile RunConnectionProfile) map[string]string {
	labels := map[string]string{
		"simops.run_id":       profile.RunID,
		"simops.worker_id":    profile.WorkerID,
		"simops.role":         string(profile.Role),
		"simops.launch_mode":  profile.LaunchMode,
		"simops.scenario_id":  profile.ScenarioID,
		"simops.worker_image": profile.WorkerImage,
	}
	if profile.WorkerKind != "" {
		labels["simops.worker_kind"] = string(profile.WorkerKind)
	}
	return labels
}

func buildRunDataPlaneConnection(cfg SimopsConfig) *RunDataPlaneConnection {
	return &RunDataPlaneConnection{
		Redpanda: RunRedpandaConnection{
			Brokers: csvValues(cfg.RedpandaBrokers),
			Topic:   strings.TrimSpace(cfg.RedpandaTopic),
		},
		Postgres: RunPostgresConnection{
			DSN: strings.TrimSpace(cfg.PostgresDSN),
		},
		Iceberg: RunIcebergConnection{
			Catalog:       strings.TrimSpace(cfg.IcebergCatalog),
			CatalogDSN:    strings.TrimSpace(cfg.IcebergCatalogDSN),
			Warehouse:     strings.TrimSpace(cfg.IcebergWarehouse),
			S3Endpoint:    strings.TrimSpace(cfg.IcebergS3Endpoint),
			S3Bucket:      strings.TrimSpace(cfg.IcebergS3Bucket),
			S3Region:      strings.TrimSpace(cfg.IcebergS3Region),
			S3AccessKeyID: strings.TrimSpace(cfg.IcebergS3AccessKeyID),
			S3SecretKey:   strings.TrimSpace(cfg.IcebergS3SecretKey),
		},
	}
}

func dockerRunContainerName(runID string, workerID string) string {
	return "simops-" + strings.TrimSpace(runID) + "-" + strings.TrimSpace(workerID)
}

var nonKubernetesNameChar = regexp.MustCompile(`[^a-z0-9-]+`)

func kubernetesRunJobName(runID string, workerID string) string {
	raw := strings.ToLower(strings.TrimSpace(runID) + "-" + strings.TrimSpace(workerID))
	raw = strings.ReplaceAll(raw, "_", "-")
	raw = strings.ReplaceAll(raw, ".", "-")
	raw = nonKubernetesNameChar.ReplaceAllString(raw, "-")
	raw = strings.Trim(raw, "-")
	if raw == "" {
		return "simops-run"
	}
	name := "simops-" + raw
	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}
	return name
}
