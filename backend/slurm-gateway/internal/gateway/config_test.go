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
