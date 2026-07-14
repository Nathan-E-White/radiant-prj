package gateway

import "time"

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
