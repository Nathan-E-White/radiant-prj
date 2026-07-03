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
