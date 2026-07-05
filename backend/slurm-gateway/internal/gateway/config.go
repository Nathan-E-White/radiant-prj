package gateway

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type SpoolerMode string

const (
	ModeMock   SpoolerMode = "mock"
	ModeSbatch SpoolerMode = "sbatch"
)

type Config struct {
	Addr              string
	Mode              SpoolerMode
	AllowedClients    map[string]struct{}
	AllowedPartitions map[string]struct{}
	AllowedScripts    map[string]struct{}
	ScriptRoot        string
	MaxNodes          int
	MaxRanks          int
	RequestTimeout    time.Duration
	SbatchBin         string
	TLSCertFile       string
	TLSKeyFile        string
	ClientCAFile      string
	RequireClientCert bool
	Simops            SimopsConfig
}

type SimopsConfig struct {
	Enabled             bool
	ControlStore        string
	PostgresDSN         string
	TelemetryLog        string
	RedpandaBrokers     string
	RedpandaTopic       string
	LaunchMode          string
	WorkerRuntime       string
	WorkerImage         string
	WorkerManifestRoot  string
	WorkerIngestBaseURL string
	WorkerFrameOverride int
	WorkerNetwork       string
	WorkerAutoRemove    bool
	MoQWebTransportURL  string
	StreamTokenTTL      time.Duration
	MaxActiveRuns       int
	IcebergCatalog      string
	IcebergCatalogDSN   string
	IcebergWarehouse    string
	IcebergS3Endpoint   string
	IcebergS3Bucket     string
	IcebergWriterMode   string
	IcebergRustCommand  string
	IcebergManifestDir  string
}

func DefaultConfig() Config {
	return Config{
		Addr:              ":8080",
		Mode:              ModeMock,
		AllowedClients:    csvSet("react-backend-client,cluster-admin"),
		AllowedPartitions: csvSet("transport,thermal,fleet,cpu-short,cpu-long,gpu"),
		AllowedScripts:    csvSet("transport-toy,thermal-margin,fleet-screen,module-rerun"),
		ScriptRoot:        ".local/slurm-scripts",
		MaxNodes:          32,
		MaxRanks:          256,
		RequestTimeout:    10 * time.Second,
		SbatchBin:         "sbatch",
		RequireClientCert: true,
		Simops: SimopsConfig{
			Enabled:             true,
			ControlStore:        "memory",
			TelemetryLog:        "memory",
			RedpandaBrokers:     "redpanda:9092",
			RedpandaTopic:       "simops.telemetry.v1",
			LaunchMode:          "resident",
			WorkerRuntime:       "contract",
			WorkerImage:         "simops-generator:latest",
			WorkerManifestRoot:  "/examples/simulation-ops",
			WorkerIngestBaseURL: "http://host.docker.internal:8080",
			WorkerNetwork:       "bridge",
			WorkerAutoRemove:    false,
			MoQWebTransportURL:  "https://127.0.0.1:9443/moq/simops",
			StreamTokenTTL:      15 * time.Minute,
			MaxActiveRuns:       8,
			IcebergCatalog:      "postgres-sql",
			IcebergWarehouse:    "s3://radiant-simops/warehouse",
			IcebergS3Endpoint:   "http://minio:9000",
			IcebergS3Bucket:     "radiant-simops",
			IcebergWriterMode:   "manifest",
			IcebergManifestDir:  "/tmp/simops-iceberg-manifests",
		},
	}
}

func LoadConfigFromEnv() (Config, error) {
	cfg := DefaultConfig()

	cfg.Addr = getenv("SLURM_GATEWAY_ADDR", cfg.Addr)
	cfg.Mode = SpoolerMode(getenv("SLURM_GATEWAY_MODE", string(cfg.Mode)))
	cfg.ScriptRoot = getenv("SLURM_GATEWAY_SCRIPT_ROOT", cfg.ScriptRoot)
	cfg.SbatchBin = getenv("SLURM_GATEWAY_SBATCH_BIN", cfg.SbatchBin)
	cfg.TLSCertFile = os.Getenv("SLURM_GATEWAY_TLS_CERT_FILE")
	cfg.TLSKeyFile = os.Getenv("SLURM_GATEWAY_TLS_KEY_FILE")
	cfg.ClientCAFile = os.Getenv("SLURM_GATEWAY_CLIENT_CA_FILE")
	cfg.Simops.ControlStore = getenv("SIMOPS_CONTROL_STORE", cfg.Simops.ControlStore)
	cfg.Simops.PostgresDSN = getenv("SIMOPS_POSTGRES_DSN", cfg.Simops.PostgresDSN)
	cfg.Simops.TelemetryLog = getenv("SIMOPS_TELEMETRY_LOG", cfg.Simops.TelemetryLog)
	cfg.Simops.RedpandaBrokers = getenv("SIMOPS_REDPANDA_BROKERS", cfg.Simops.RedpandaBrokers)
	cfg.Simops.RedpandaTopic = getenv("SIMOPS_REDPANDA_TOPIC", cfg.Simops.RedpandaTopic)
	cfg.Simops.LaunchMode = getenv("SIMOPS_LAUNCH_MODE", cfg.Simops.LaunchMode)
	cfg.Simops.WorkerRuntime = getenv("SIMOPS_WORKER_RUNTIME", cfg.Simops.WorkerRuntime)
	cfg.Simops.WorkerImage = getenv("SIMOPS_WORKER_IMAGE", cfg.Simops.WorkerImage)
	cfg.Simops.WorkerManifestRoot = getenv("SIMOPS_WORKER_MANIFEST_ROOT", cfg.Simops.WorkerManifestRoot)
	cfg.Simops.WorkerIngestBaseURL = getenv("SIMOPS_WORKER_INGEST_BASE_URL", cfg.Simops.WorkerIngestBaseURL)
	cfg.Simops.WorkerNetwork = getenv("SIMOPS_WORKER_NETWORK", cfg.Simops.WorkerNetwork)
	cfg.Simops.MoQWebTransportURL = getenv("SIMOPS_MOQ_WEBTRANSPORT_URL", cfg.Simops.MoQWebTransportURL)
	cfg.Simops.IcebergCatalog = getenv("SIMOPS_ICEBERG_CATALOG", cfg.Simops.IcebergCatalog)
	cfg.Simops.IcebergCatalogDSN = getenv("SIMOPS_ICEBERG_CATALOG_DSN", cfg.Simops.IcebergCatalogDSN)
	cfg.Simops.IcebergWarehouse = getenv("SIMOPS_ICEBERG_WAREHOUSE", cfg.Simops.IcebergWarehouse)
	cfg.Simops.IcebergS3Endpoint = getenv("SIMOPS_ICEBERG_S3_ENDPOINT", cfg.Simops.IcebergS3Endpoint)
	cfg.Simops.IcebergS3Bucket = getenv("SIMOPS_ICEBERG_S3_BUCKET", cfg.Simops.IcebergS3Bucket)
	cfg.Simops.IcebergWriterMode = getenv("SIMOPS_ICEBERG_WRITER_MODE", cfg.Simops.IcebergWriterMode)
	cfg.Simops.IcebergRustCommand = strings.TrimSpace(os.Getenv("SIMOPS_ICEBERG_RUST_CMD"))
	cfg.Simops.IcebergManifestDir = getenv("SIMOPS_ICEBERG_MANIFEST_DIR", cfg.Simops.IcebergManifestDir)

	if raw := strings.TrimSpace(os.Getenv("SLURM_GATEWAY_ALLOWED_CLIENTS")); raw != "" {
		cfg.AllowedClients = csvSet(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("SLURM_GATEWAY_ALLOWED_PARTITIONS")); raw != "" {
		cfg.AllowedPartitions = csvSet(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("SLURM_GATEWAY_ALLOWED_SCRIPTS")); raw != "" {
		cfg.AllowedScripts = csvSet(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("SLURM_GATEWAY_MAX_NODES")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return cfg, fmt.Errorf("SLURM_GATEWAY_MAX_NODES must be an integer: %w", err)
		}
		cfg.MaxNodes = value
	}
	if raw := strings.TrimSpace(os.Getenv("SLURM_GATEWAY_MAX_RANKS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return cfg, fmt.Errorf("SLURM_GATEWAY_MAX_RANKS must be an integer: %w", err)
		}
		cfg.MaxRanks = value
	}
	if raw := strings.TrimSpace(os.Getenv("SLURM_GATEWAY_REQUEST_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil {
			return cfg, fmt.Errorf("SLURM_GATEWAY_REQUEST_TIMEOUT must be a duration: %w", err)
		}
		cfg.RequestTimeout = value
	}
	if raw := strings.TrimSpace(os.Getenv("SLURM_GATEWAY_REQUIRE_CLIENT_CERT")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("SLURM_GATEWAY_REQUIRE_CLIENT_CERT must be boolean: %w", err)
		}
		cfg.RequireClientCert = value
	}
	if raw := strings.TrimSpace(os.Getenv("SIMOPS_ENABLED")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("SIMOPS_ENABLED must be boolean: %w", err)
		}
		cfg.Simops.Enabled = value
	}
	if raw := strings.TrimSpace(os.Getenv("SIMOPS_STREAM_TOKEN_TTL")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil {
			return cfg, fmt.Errorf("SIMOPS_STREAM_TOKEN_TTL must be a duration: %w", err)
		}
		cfg.Simops.StreamTokenTTL = value
	}
	if raw := strings.TrimSpace(os.Getenv("SIMOPS_MAX_ACTIVE_RUNS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return cfg, fmt.Errorf("SIMOPS_MAX_ACTIVE_RUNS must be an integer: %w", err)
		}
		cfg.Simops.MaxActiveRuns = value
	}
	if raw := strings.TrimSpace(os.Getenv("SIMOPS_WORKER_AUTO_REMOVE")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("SIMOPS_WORKER_AUTO_REMOVE must be boolean: %w", err)
		}
		cfg.Simops.WorkerAutoRemove = value
	}
	if raw := strings.TrimSpace(os.Getenv("SIMOPS_WORKER_FRAME_OVERRIDE")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return cfg, fmt.Errorf("SIMOPS_WORKER_FRAME_OVERRIDE must be an integer: %w", err)
		}
		cfg.Simops.WorkerFrameOverride = value
	}

	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	switch c.Mode {
	case ModeMock, ModeSbatch:
	default:
		return fmt.Errorf("unsupported spooler mode %q", c.Mode)
	}
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("gateway address is required")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}
	if c.MaxNodes < 1 {
		return fmt.Errorf("max nodes must be at least 1")
	}
	if c.MaxRanks < 1 {
		return fmt.Errorf("max ranks must be at least 1")
	}
	if len(c.AllowedClients) == 0 {
		return fmt.Errorf("at least one allowed client identity is required")
	}
	if len(c.AllowedPartitions) == 0 {
		return fmt.Errorf("at least one allowed partition is required")
	}
	if len(c.AllowedScripts) == 0 {
		return fmt.Errorf("at least one allowed script is required")
	}
	if strings.TrimSpace(c.ScriptRoot) == "" {
		return fmt.Errorf("script root is required")
	}
	if c.Mode == ModeSbatch && strings.TrimSpace(c.SbatchBin) == "" {
		return fmt.Errorf("sbatch binary is required in sbatch mode")
	}
	if c.TLSCertFile != "" || c.TLSKeyFile != "" || c.ClientCAFile != "" {
		if c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" {
			return fmt.Errorf("TLS cert, TLS key, and client CA files must be supplied together")
		}
	}
	if err := c.Simops.Validate(); err != nil {
		return err
	}
	return nil
}

func (c SimopsConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	switch c.ControlStore {
	case "memory", "postgres":
	default:
		return fmt.Errorf("unsupported SIMOPS_CONTROL_STORE %q", c.ControlStore)
	}
	if c.ControlStore == "postgres" && strings.TrimSpace(c.PostgresDSN) == "" {
		return fmt.Errorf("SIMOPS_POSTGRES_DSN is required when SIMOPS_CONTROL_STORE=postgres")
	}
	switch c.TelemetryLog {
	case "memory", "redpanda":
	default:
		return fmt.Errorf("unsupported SIMOPS_TELEMETRY_LOG %q", c.TelemetryLog)
	}
	if c.TelemetryLog == "redpanda" {
		if strings.TrimSpace(c.RedpandaBrokers) == "" {
			return fmt.Errorf("SIMOPS_REDPANDA_BROKERS is required when SIMOPS_TELEMETRY_LOG=redpanda")
		}
		if strings.TrimSpace(c.RedpandaTopic) == "" {
			return fmt.Errorf("SIMOPS_REDPANDA_TOPIC is required when SIMOPS_TELEMETRY_LOG=redpanda")
		}
	}
	switch c.LaunchMode {
	case "resident", "spawn", "auto":
	default:
		return fmt.Errorf("unsupported SIMOPS_LAUNCH_MODE %q", c.LaunchMode)
	}
	if strings.TrimSpace(c.MoQWebTransportURL) == "" {
		return fmt.Errorf("SIMOPS_MOQ_WEBTRANSPORT_URL is required")
	}
	if c.StreamTokenTTL <= 0 {
		return fmt.Errorf("SIMOPS_STREAM_TOKEN_TTL must be positive")
	}
	if c.MaxActiveRuns < 1 {
		return fmt.Errorf("SIMOPS_MAX_ACTIVE_RUNS must be at least 1")
	}
	switch c.WorkerRuntime {
	case "contract", "docker":
	default:
		return fmt.Errorf("unsupported SIMOPS_WORKER_RUNTIME %q", c.WorkerRuntime)
	}
	if c.WorkerRuntime == "docker" {
		if strings.TrimSpace(c.WorkerImage) == "" {
			return fmt.Errorf("SIMOPS_WORKER_IMAGE is required when SIMOPS_WORKER_RUNTIME=docker")
		}
		if strings.TrimSpace(c.WorkerManifestRoot) == "" {
			return fmt.Errorf("SIMOPS_WORKER_MANIFEST_ROOT is required when SIMOPS_WORKER_RUNTIME=docker")
		}
		if strings.TrimSpace(c.WorkerIngestBaseURL) == "" {
			return fmt.Errorf("SIMOPS_WORKER_INGEST_BASE_URL is required when SIMOPS_WORKER_RUNTIME=docker")
		}
		if c.WorkerFrameOverride < 0 {
			return fmt.Errorf("SIMOPS_WORKER_FRAME_OVERRIDE must be zero or positive")
		}
	}
	switch c.IcebergCatalog {
	case "postgres-sql", "rest", "filesystem":
	default:
		return fmt.Errorf("unsupported SIMOPS_ICEBERG_CATALOG %q", c.IcebergCatalog)
	}
	if c.IcebergCatalog == "postgres-sql" && c.ControlStore == "postgres" && strings.TrimSpace(c.IcebergCatalogDSN) == "" {
		return fmt.Errorf("SIMOPS_ICEBERG_CATALOG_DSN is required when using the Postgres Iceberg SQL catalog")
	}
	if strings.TrimSpace(c.IcebergWarehouse) == "" {
		return fmt.Errorf("SIMOPS_ICEBERG_WAREHOUSE is required")
	}
	if strings.TrimSpace(c.IcebergS3Bucket) == "" {
		return fmt.Errorf("SIMOPS_ICEBERG_S3_BUCKET is required")
	}
	if strings.TrimSpace(c.IcebergManifestDir) == "" {
		return fmt.Errorf("SIMOPS_ICEBERG_MANIFEST_DIR is required")
	}
	switch c.IcebergWriterMode {
	case "manifest", "stub", "external", "disabled":
	default:
		return fmt.Errorf("unsupported SIMOPS_ICEBERG_WRITER_MODE %q", c.IcebergWriterMode)
	}
	if c.IcebergWriterMode == "external" && strings.TrimSpace(c.IcebergRustCommand) == "" {
		return fmt.Errorf("SIMOPS_ICEBERG_RUST_CMD is required when SIMOPS_ICEBERG_WRITER_MODE=external")
	}
	return nil
}

func (c Config) TLSEnabled() bool {
	return c.TLSCertFile != "" && c.TLSKeyFile != "" && c.ClientCAFile != ""
}

func getenv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func csvSet(raw string) map[string]struct{} {
	values := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value != "" {
			values[value] = struct{}{}
		}
	}
	return values
}
