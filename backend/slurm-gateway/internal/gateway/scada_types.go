package gateway

import "time"

type WorkbenchValueBasis string

const (
	WorkbenchValueMeasured  WorkbenchValueBasis = "measured"
	WorkbenchValueImputed   WorkbenchValueBasis = "imputed"
	WorkbenchValueSimulated WorkbenchValueBasis = "simulated"
)

type ScadaSignalKind string

const (
	ScadaSignalFlux            ScadaSignalKind = "flux"
	ScadaSignalTemperature     ScadaSignalKind = "temperature"
	ScadaSignalPressure        ScadaSignalKind = "pressure"
	ScadaSignalFlow            ScadaSignalKind = "flow"
	ScadaSignalActuatorState   ScadaSignalKind = "actuatorState"
	ScadaSignalElectricalState ScadaSignalKind = "electricalState"
	ScadaSignalComms           ScadaSignalKind = "comms"
)

type ScadaTelemetryFrame struct {
	SchemaVersion   string              `json:"schemaVersion"`
	SourceID        string              `json:"sourceId"`
	ReactorID       string              `json:"reactorId,omitempty"`
	TagID           string              `json:"tagId"`
	AssetID         string              `json:"assetId"`
	SignalKind      ScadaSignalKind     `json:"signalKind"`
	SampledAt       time.Time           `json:"sampledAt"`
	ObservedAt      time.Time           `json:"observedAt"`
	Sequence        uint64              `json:"sequence"`
	Unit            string              `json:"unit"`
	Value           map[string]any      `json:"value"`
	Quality         string              `json:"quality"`
	ValueBasis      WorkbenchValueBasis `json:"valueBasis"`
	SyntheticStatus string              `json:"syntheticStatus"`
}

type ScadaResidentSourceDeclaration struct {
	SchemaVersion   string           `json:"schemaVersion"`
	SourceID        string           `json:"sourceId"`
	ReactorID       string           `json:"reactorId,omitempty"`
	DisplayName     string           `json:"displayName"`
	Lifecycle       string           `json:"lifecycle"`
	SyntheticStatus string           `json:"syntheticStatus"`
	Ingest          ScadaIngest      `json:"ingest"`
	Tags            []ScadaSourceTag `json:"tags"`
}

type ScadaIngest struct {
	Topic        string `json:"topic"`
	EndpointKind string `json:"endpointKind"`
}

type ScadaSourceTag struct {
	SourceID   string              `json:"sourceId,omitempty"`
	ReactorID  string              `json:"reactorId,omitempty"`
	TagID      string              `json:"tagId"`
	AssetID    string              `json:"assetId"`
	SignalKind ScadaSignalKind     `json:"signalKind"`
	Unit       string              `json:"unit"`
	ValueBasis WorkbenchValueBasis `json:"valueBasis"`
}
