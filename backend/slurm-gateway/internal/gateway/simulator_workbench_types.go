package gateway

import (
	"fmt"
	"time"
)

// ValidateWorkbenchSnapshot is the canonical cross-runtime coherence check.
// It is intentionally narrow: detailed payload validation stays at each
// projection boundary, while every consumer must agree on this envelope's
// generation and Value Basis invariants.
func ValidateWorkbenchSnapshot(snapshot WorkbenchSnapshot) error {
	if snapshot.State.SnapshotGeneration != snapshot.Generation {
		return fmt.Errorf("workbench state and Snapshot generation do not match")
	}
	for _, frame := range snapshot.Measured {
		if frame.ValueBasis != WorkbenchValueMeasured {
			return fmt.Errorf("measured frame %s has invalid Value Basis %q", frame.TagID, frame.ValueBasis)
		}
	}
	for _, frame := range snapshot.Results {
		if frame.ValueBasis != WorkbenchValueSimulated {
			return fmt.Errorf("result frame %s has invalid Value Basis %q", frame.RunID, frame.ValueBasis)
		}
	}
	for _, entity := range snapshot.Twin.Entities {
		for _, value := range entity.Values {
			if value.ValueBasis != WorkbenchValueMeasured && value.ValueBasis != WorkbenchValueImputed && value.ValueBasis != WorkbenchValueSimulated {
				return fmt.Errorf("Twin value %s has invalid Value Basis %q", value.ValueID, value.ValueBasis)
			}
		}
	}
	for _, lineage := range snapshot.Lineage {
		if lineage.ValueBasis != WorkbenchValueMeasured && lineage.ValueBasis != WorkbenchValueImputed && lineage.ValueBasis != WorkbenchValueSimulated {
			return fmt.Errorf("lineage %s has invalid Value Basis %q", lineage.ValueID, lineage.ValueBasis)
		}
	}
	return nil
}

type SimulatorWorkbenchState struct {
	SchemaVersion        string                          `json:"schemaVersion"`
	GeneratedAt          time.Time                       `json:"generatedAt"`
	SnapshotGeneration   uint64                          `json:"snapshotGeneration"`
	ScenarioID           string                          `json:"scenarioId"`
	ValueBasisSummary    map[WorkbenchValueBasis]int     `json:"valueBasisSummary"`
	MeasuredStateRefs    []string                        `json:"measuredStateRefs"`
	TwinStateRef         string                          `json:"twinStateRef"`
	LineageRefs          []string                        `json:"lineageRefs"`
	ActiveSimulationRuns []WorkbenchSimulationRunSummary `json:"activeSimulationRuns"`
	Panels               []WorkbenchPanelSummary         `json:"panels"`
}

type WorkbenchSimulationRunSummary struct {
	RunID          string              `json:"runId"`
	ScenarioID     string              `json:"scenarioId"`
	Lifecycle      string              `json:"lifecycle"`
	ValueBasis     WorkbenchValueBasis `json:"valueBasis"`
	Health         string              `json:"health"`
	ArtifactStatus string              `json:"artifactStatus"`
}

type WorkbenchPanelSummary struct {
	PanelID    string              `json:"panelId"`
	Title      string              `json:"title"`
	ValueBasis WorkbenchValueBasis `json:"valueBasis"`
}
