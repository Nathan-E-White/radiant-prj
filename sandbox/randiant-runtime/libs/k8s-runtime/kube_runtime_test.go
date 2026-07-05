package k8sruntime

import (
	"testing"
)

func TestBuildVarFlagsIsDeterministic(t *testing.T) {
	vars := map[string]string{"b": "2", "a": "1"}
	flags := buildVarFlags(vars)
	expected := []string{"-var", "a=1", "-var", "b=2"}
	if len(flags) != len(expected) {
		t.Fatalf("unexpected length: %d", len(flags))
	}
	for i := range expected {
		if flags[i] != expected[i] {
			t.Fatalf("unexpected flag order: got %#v", flags)
		}
	}
}
