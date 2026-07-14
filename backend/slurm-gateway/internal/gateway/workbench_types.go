package gateway

import (
	"encoding/json"
	"time"
)

const (
	WorkbenchScadaSchemaVersion      = "scada.telemetry.v1"
	WorkbenchSourceSchemaVersion     = "scada.resident-source-declaration.v1"
	WorkbenchResultSchemaVersion     = "simops.result.v1"
	WorkbenchTwinStateSchemaVersion  = "digital-twin.state.v1"
	WorkbenchLineageSchemaVersion    = "digital-twin.lineage.v1"
	WorkbenchSyntheticPublicStandin  = "public-safe-standin"
	WorkbenchDefaultTwinID           = "TWIN-PUBLIC-SAFE-001"
	WorkbenchDefaultTwinModelID      = "MODEL-TWIN-THERMAL-V0"
	WorkbenchImputedCoreMarginValue  = "VAL-IMPUTED-CORE-MARGIN"
	WorkbenchSimulatedMarginValue    = "VAL-SIMULATED-FORECAST-MARGIN"
	WorkbenchCoreMarginLineage       = "LIN-CORE-MARGIN"
	WorkbenchSimulatedMarginLineage  = "LIN-SIM-MARGIN"
	WorkbenchMeasuredValueIDPrefix   = "VAL-MEASURED-"
	WorkbenchMeasuredLineageIDPrefix = "LIN-MEASURED-"
)

type SimopsResultFrame struct {
	SchemaVersion    string                  `json:"schemaVersion"`
	RunID            string                  `json:"runId"`
	ScenarioID       string                  `json:"scenarioId"`
	WorkerID         string                  `json:"workerId"`
	WorkerKind       SimopsWorkerKind        `json:"workerKind"`
	Sequence         uint64                  `json:"sequence"`
	ProducedAt       string                  `json:"producedAt"`
	ReceivedAt       string                  `json:"receivedAt,omitempty"`
	ResultType       string                  `json:"resultType"`
	ModelID          string                  `json:"modelId"`
	InputWindow      SimopsResultInputWindow `json:"inputWindow"`
	ValueBasis       WorkbenchValueBasis     `json:"valueBasis"`
	SyntheticStatus  string                  `json:"syntheticStatus"`
	Values           []SimopsResultValue     `json:"values"`
	LineageInputs    []TwinLineageInput      `json:"lineageInputs,omitempty"`
	LineageArtifacts []TwinLineageArtifact   `json:"lineageArtifacts,omitempty"`
}

type SimopsResultInputWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type SimopsResultValue struct {
	ResultID   string          `json:"resultId"`
	EntityID   string          `json:"entityId"`
	ValueID    string          `json:"valueId"`
	Label      string          `json:"label"`
	Unit       string          `json:"unit"`
	Value      json.RawMessage `json:"value"`
	Confidence float64         `json:"confidence"`
}

type SimopsResultBatch struct {
	Results []SimopsResultFrame `json:"results"`
}

type ScadaTelemetryBatch struct {
	Frames []ScadaTelemetryFrame `json:"frames"`
}

type ScadaProjection struct {
	ObservedAt        time.Time
	SampledAt         time.Time
	Frame             ScadaTelemetryFrame
	Raw               json.RawMessage
	RedpandaTopic     string
	RedpandaPartition int
	RedpandaOffset    int64
}

type SimopsResultProjection struct {
	ProducedAt        time.Time
	ReceivedAt        time.Time
	Frame             SimopsResultFrame
	Raw               json.RawMessage
	RedpandaTopic     string
	RedpandaPartition int
	RedpandaOffset    int64
}

type TwinStateProjection struct {
	AsOf              time.Time
	State             DigitalTwinState
	Lineage           []DigitalTwinValueLineage
	LineagePresent    bool
	PublicationID     string
	PublicationSource WorkbenchProjectionPosition
	Raw               json.RawMessage
	RedpandaTopic     string
	RedpandaPartition int
	RedpandaOffset    int64
}

type TwinStateTransition struct {
	State   DigitalTwinState          `json:"state"`
	Lineage []DigitalTwinValueLineage `json:"lineage"`
}

// WorkbenchSnapshot is one committed read moment. Generation advances exactly
// once for each committed transition that changes a returned component; replayed
// or rolled-back transitions do not advance it. State and all payloads are from
// that same generation.
type WorkbenchSnapshot struct {
	Generation uint64                    `json:"generation"`
	State      SimulatorWorkbenchState   `json:"state"`
	Measured   []ScadaTelemetryFrame     `json:"measured"`
	Twin       DigitalTwinState          `json:"twin"`
	Lineage    []DigitalTwinValueLineage `json:"lineage"`
	Results    []SimopsResultFrame       `json:"results"`
}
