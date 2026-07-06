package gateway

import "time"

type DigitalTwinState struct {
	SchemaVersion string              `json:"schemaVersion"`
	TwinID        string              `json:"twinId"`
	AsOf          time.Time           `json:"asOf"`
	Entities      []DigitalTwinEntity `json:"entities"`
}

type DigitalTwinEntity struct {
	EntityID    string             `json:"entityId"`
	DisplayName string             `json:"displayName"`
	Values      []DigitalTwinValue `json:"values"`
}

type DigitalTwinValue struct {
	ValueID    string              `json:"valueId"`
	Label      string              `json:"label"`
	ValueBasis WorkbenchValueBasis `json:"valueBasis"`
	Unit       string              `json:"unit"`
	Value      map[string]any      `json:"value"`
	Confidence float64             `json:"confidence"`
	Freshness  TwinFreshness       `json:"freshness"`
	LineageID  string              `json:"lineageId"`
	SourceIDs  []string            `json:"sourceIds"`
}

type TwinFreshness struct {
	AgeSec int    `json:"ageSec"`
	Status string `json:"status"`
}

type DigitalTwinValueLineage struct {
	SchemaVersion   string                `json:"schemaVersion"`
	LineageID       string                `json:"lineageId"`
	ValueID         string                `json:"valueId"`
	ValueBasis      WorkbenchValueBasis   `json:"valueBasis"`
	Inputs          []TwinLineageInput    `json:"inputs"`
	ProcessingSteps []string              `json:"processingSteps"`
	Artifacts       []TwinLineageArtifact `json:"artifacts"`
}

type TwinLineageInput struct {
	SourceKind string              `json:"sourceKind"`
	SourceID   string              `json:"sourceId"`
	ValueBasis WorkbenchValueBasis `json:"valueBasis"`
}

type TwinLineageArtifact struct {
	ArtifactID string `json:"artifactId"`
	Path       string `json:"path"`
	MediaType  string `json:"mediaType"`
}
