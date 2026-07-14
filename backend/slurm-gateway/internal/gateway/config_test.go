package gateway

import (
	"testing"
	"time"
)

func TestConfigValidateRejectsUnsafeAllowedScripts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AllowedScripts = csvSet("transport-toy,../outside")

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected unsafe allowed script to be rejected")
	}
}

func TestSimopsConfigRequiresPositiveLifecycleRecoveryTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Simops.LifecycleRecoveryTimeout = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected zero lifecycle recovery timeout to be rejected")
	}

	t.Setenv("SIMOPS_LIFECYCLE_RECOVERY_TIMEOUT", "750ms")
	loaded, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Simops.LifecycleRecoveryTimeout != 750*time.Millisecond {
		t.Fatalf("unexpected lifecycle recovery timeout %v", loaded.Simops.LifecycleRecoveryTimeout)
	}
}

func TestReactorTelemetryConfigLoadsOnlyAcceptedBounds(t *testing.T) {
	t.Setenv("REACTOR_TELEMETRY_ENABLED", "true")
	t.Setenv("REACTOR_TELEMETRY_RUNTIME", "docker")
	t.Setenv("REACTOR_TELEMETRY_WORKERS_PER_SET", "3")
	t.Setenv("REACTOR_TELEMETRY_MAX_REACTORS_PER_SESSION", "4")
	t.Setenv("REACTOR_TELEMETRY_SESSION_TTL", "24h")
	t.Setenv("REACTOR_TELEMETRY_CLEANUP_TIMEOUT", "5m")
	loaded, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("load reactor telemetry config: %v", err)
	}
	if !loaded.ReactorTelemetry.Enabled || loaded.ReactorTelemetry.Runtime != "docker" || loaded.ReactorTelemetry.WorkersPerSet != 3 {
		t.Fatalf("unexpected reactor telemetry config: %#v", loaded.ReactorTelemetry)
	}

	t.Setenv("REACTOR_TELEMETRY_WORKERS_PER_SET", "4")
	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatal("configuration raised the accepted worker cap")
	}
}
