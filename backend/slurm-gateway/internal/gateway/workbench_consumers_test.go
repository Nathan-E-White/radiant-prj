package gateway

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildTwinStateFromDataProducesImputedStateFromMeasuredAndSimulatedInputs(t *testing.T) {
	measured := []ScadaTelemetryFrame{scadaFrameFixture()}
	result := simopsResultFixture("RUN-TWIN-001")
	asOf := time.Date(2026, 7, 6, 15, 1, 0, 0, time.UTC)

	state, lineage := BuildTwinStateFromData(measured, result, asOf)

	if state.SchemaVersion != WorkbenchTwinStateSchemaVersion {
		t.Fatalf("unexpected schema version %q", state.SchemaVersion)
	}
	counts := map[WorkbenchValueBasis]int{}
	for _, entity := range state.Entities {
		for _, value := range entity.Values {
			counts[value.ValueBasis]++
		}
	}
	if counts[WorkbenchValueMeasured] == 0 || counts[WorkbenchValueSimulated] == 0 || counts[WorkbenchValueImputed] == 0 {
		t.Fatalf("expected measured, simulated, and imputed values, got %#v", counts)
	}

	var imputedLineage DigitalTwinValueLineage
	for _, record := range lineage {
		if record.ValueID == WorkbenchImputedCoreMarginValue {
			imputedLineage = record
		}
	}
	if imputedLineage.ValueBasis != WorkbenchValueImputed {
		t.Fatalf("expected imputed lineage, got %#v", imputedLineage)
	}
	seenMeasured := false
	seenSimulated := false
	for _, input := range imputedLineage.Inputs {
		seenMeasured = seenMeasured || input.ValueBasis == WorkbenchValueMeasured
		seenSimulated = seenSimulated || input.ValueBasis == WorkbenchValueSimulated
	}
	if !seenMeasured || !seenSimulated {
		raw, _ := json.MarshalIndent(imputedLineage, "", "  ")
		t.Fatalf("expected measured and simulated lineage inputs, got %s", string(raw))
	}
}
