package gateway

import (
	"strings"
	"testing"
	"time"
)

func TestRunConnectionProfilesForOrdinaryWorkers(t *testing.T) {
	cfg := testRunConnectionProfileConfig()
	run := testRunConnectionProfileRecord()

	profiles, err := BuildRunWorkerConnectionProfiles(cfg, run, []SimopsWorkerKind{SimopsWorkerScheduler, SimopsWorkerStorage})
	if err != nil {
		t.Fatalf("build profiles: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected two profiles, got %d", len(profiles))
	}

	profile := profiles[0]
	if profile.Role != RunConnectionRoleOrdinaryWorker {
		t.Fatalf("expected ordinary worker role, got %q", profile.Role)
	}
	if profile.RunID != "RUN-PROFILE-001" || profile.ScenarioID != "scheduler-drift" {
		t.Fatalf("unexpected run identity: %#v", profile)
	}
	if profile.WorkerID != "scheduler-01" || profile.WorkerKind != SimopsWorkerScheduler {
		t.Fatalf("unexpected worker identity: %#v", profile)
	}
	if profile.Gateway.IngestURL != "http://slurm-gateway:8080/internal/simops/runs/RUN-PROFILE-001/ingest" {
		t.Fatalf("unexpected ingest URL %q", profile.Gateway.IngestURL)
	}
	if profile.Gateway.ResultIngestURL != "http://slurm-gateway:8080/internal/simops/runs/RUN-PROFILE-001/results" {
		t.Fatalf("unexpected result ingest URL %q", profile.Gateway.ResultIngestURL)
	}
	if profile.WorkerImage != "radiant-simops-generator:test" {
		t.Fatalf("unexpected worker image %q", profile.WorkerImage)
	}
	if profile.ManifestPath != "/examples/simulation-ops/run-manifest.scheduler-drift.json" {
		t.Fatalf("unexpected manifest path %q", profile.ManifestPath)
	}
	if profile.DataPlane != nil {
		t.Fatalf("ordinary worker profile must not include data-plane refs: %#v", profile.DataPlane)
	}
	if profile.Labels["simops.worker_kind"] != "scheduler" || profile.Labels["simops.role"] != "ordinary-worker" {
		t.Fatalf("missing deterministic worker labels: %#v", profile.Labels)
	}
}

func TestTrustedRunConnectionProfileIncludesDataPlaneRefs(t *testing.T) {
	cfg := testRunConnectionProfileConfig()
	run := testRunConnectionProfileRecord()

	profile, err := BuildTrustedRunConnectionProfile(cfg, run, RunConnectionRoleIcebergWriter)
	if err != nil {
		t.Fatalf("build trusted profile: %v", err)
	}
	if profile.Role != RunConnectionRoleIcebergWriter {
		t.Fatalf("unexpected role %q", profile.Role)
	}
	if profile.DataPlane == nil {
		t.Fatalf("trusted profile should include data-plane refs")
	}
	if strings.Join(profile.DataPlane.Redpanda.Brokers, ",") != "redpanda:9092,redpanda-backup:9092" {
		t.Fatalf("unexpected Redpanda brokers %#v", profile.DataPlane.Redpanda.Brokers)
	}
	if profile.DataPlane.Redpanda.Topic != "simops.telemetry.v1" {
		t.Fatalf("unexpected Redpanda topic %q", profile.DataPlane.Redpanda.Topic)
	}
	if profile.DataPlane.Postgres.DSN != "postgres://radiant:radiant@postgres:5432/radiant?sslmode=disable" {
		t.Fatalf("unexpected Postgres DSN %q", profile.DataPlane.Postgres.DSN)
	}
	if profile.DataPlane.Iceberg.Warehouse != "s3://radiant-simops/warehouse" {
		t.Fatalf("unexpected Iceberg warehouse %q", profile.DataPlane.Iceberg.Warehouse)
	}
	if profile.DataPlane.Iceberg.S3AccessKeyID != "radiant" || profile.DataPlane.Iceberg.S3SecretKey != "radiant-password" {
		t.Fatalf("unexpected Iceberg S3 credential refs: %#v", profile.DataPlane.Iceberg)
	}
}

func TestOrdinaryRunConnectionProfileExcludesDataPlaneCredentials(t *testing.T) {
	cfg := testRunConnectionProfileConfig()
	run := testRunConnectionProfileRecord()

	profile, err := BuildRunWorkerConnectionProfile(cfg, run, SimopsWorkerFabric)
	if err != nil {
		t.Fatalf("build ordinary profile: %v", err)
	}
	if profile.DataPlane != nil {
		t.Fatalf("ordinary worker received data-plane refs: %#v", profile.DataPlane)
	}
	for _, label := range profile.Labels {
		for _, forbidden := range []string{"postgres://", "radiant-password", "redpanda:9092", "s3://radiant-simops"} {
			if strings.Contains(label, forbidden) {
				t.Fatalf("ordinary worker label leaked %q in %#v", forbidden, profile.Labels)
			}
		}
	}
}

func TestRunConnectionProfileMapsDockerAndKubernetesRuntimeNeeds(t *testing.T) {
	cfg := testRunConnectionProfileConfig()
	run := testRunConnectionProfileRecord()

	profile, err := BuildRunWorkerConnectionProfile(cfg, run, SimopsWorkerBurst)
	if err != nil {
		t.Fatalf("build profile: %v", err)
	}
	if profile.Runtime.Docker.Network != "radiant-simops-local" {
		t.Fatalf("unexpected Docker network %q", profile.Runtime.Docker.Network)
	}
	if profile.Runtime.Docker.ContainerName != "simops-RUN-PROFILE-001-burst-01" {
		t.Fatalf("unexpected Docker container name %q", profile.Runtime.Docker.ContainerName)
	}
	if profile.Runtime.Kubernetes.Namespace != "radiant-simops" {
		t.Fatalf("unexpected Kubernetes namespace %q", profile.Runtime.Kubernetes.Namespace)
	}
	if profile.Runtime.Kubernetes.JobName != "simops-run-profile-001-burst-01" {
		t.Fatalf("unexpected Kubernetes job name %q", profile.Runtime.Kubernetes.JobName)
	}
	if profile.Cleanup.TTLSecondsAfterFinished != 600 {
		t.Fatalf("unexpected cleanup TTL %d", profile.Cleanup.TTLSecondsAfterFinished)
	}
}

func TestRunConnectionProfileRejectsIncompleteConfig(t *testing.T) {
	run := testRunConnectionProfileRecord()

	cases := []struct {
		name    string
		mutate  func(*SimopsConfig, *SimopsRunRecord)
		message string
	}{
		{
			name: "missing image",
			mutate: func(cfg *SimopsConfig, _ *SimopsRunRecord) {
				cfg.WorkerImage = ""
			},
			message: "SIMOPS_WORKER_IMAGE",
		},
		{
			name: "missing ingest base url",
			mutate: func(cfg *SimopsConfig, _ *SimopsRunRecord) {
				cfg.WorkerIngestBaseURL = ""
			},
			message: "SIMOPS_WORKER_INGEST_BASE_URL",
		},
		{
			name: "missing namespace",
			mutate: func(cfg *SimopsConfig, _ *SimopsRunRecord) {
				cfg.WorkerKubernetesNamespace = ""
			},
			message: "SIMOPS_WORKER_KUBERNETES_NAMESPACE",
		},
		{
			name: "missing ingest token",
			mutate: func(_ *SimopsConfig, run *SimopsRunRecord) {
				run.IngestToken = ""
			},
			message: "ingest token",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testRunConnectionProfileConfig()
			scopedRun := run
			tc.mutate(&cfg, &scopedRun)
			_, err := BuildRunWorkerConnectionProfile(cfg, scopedRun, SimopsWorkerScheduler)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("expected %q error, got %v", tc.message, err)
			}
		})
	}
}

func testRunConnectionProfileConfig() SimopsConfig {
	cfg := DefaultConfig().Simops
	cfg.WorkerRuntime = "docker"
	cfg.WorkerImage = "radiant-simops-generator:test"
	cfg.WorkerManifestRoot = "/examples/simulation-ops"
	cfg.WorkerIngestBaseURL = "http://slurm-gateway:8080/"
	cfg.WorkerNetwork = "radiant-simops-local"
	cfg.WorkerKubernetesNamespace = "radiant-simops"
	cfg.WorkerAutoRemove = true
	cfg.WorkerCleanupTTL = 10 * time.Minute
	cfg.RedpandaBrokers = "redpanda:9092, redpanda-backup:9092"
	cfg.RedpandaTopic = "simops.telemetry.v1"
	cfg.PostgresDSN = "postgres://radiant:radiant@postgres:5432/radiant?sslmode=disable"
	cfg.IcebergCatalogDSN = "postgres://radiant:radiant@postgres:5432/radiant?sslmode=disable"
	cfg.IcebergWarehouse = "s3://radiant-simops/warehouse"
	cfg.IcebergS3Endpoint = "http://minio:9000"
	cfg.IcebergS3Bucket = "radiant-simops"
	cfg.IcebergS3Region = "local"
	cfg.IcebergS3AccessKeyID = "radiant"
	cfg.IcebergS3SecretKey = "radiant-password"
	return cfg
}

func testRunConnectionProfileRecord() SimopsRunRecord {
	return SimopsRunRecord{
		RunID:           "RUN-PROFILE-001",
		ScenarioID:      "scheduler-drift",
		LaunchMode:      "auto",
		RuntimeLimitSec: 120,
		IngestToken:     "ingest-token",
	}
}
