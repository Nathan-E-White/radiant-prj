package gateway

import "testing"

func TestSimulatorWorkbenchValueBasisConstantsStaySeparated(t *testing.T) {
	values := []WorkbenchValueBasis{
		WorkbenchValueMeasured,
		WorkbenchValueImputed,
		WorkbenchValueSimulated,
	}
	seen := map[WorkbenchValueBasis]bool{}
	for _, value := range values {
		if seen[value] {
			t.Fatalf("duplicate value basis %q", value)
		}
		seen[value] = true
	}
	if !seen["measured"] || !seen["imputed"] || !seen["simulated"] {
		t.Fatalf("missing expected value-basis constants: %#v", seen)
	}
}
