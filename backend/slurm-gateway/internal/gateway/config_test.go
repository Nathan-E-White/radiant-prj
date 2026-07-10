package gateway

import "testing"

func TestConfigValidateRejectsUnsafeAllowedScripts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AllowedScripts = csvSet("transport-toy,../outside")

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected unsafe allowed script to be rejected")
	}
}
