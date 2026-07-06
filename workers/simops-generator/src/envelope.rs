use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::manifest::{
    ManifestArtifact, PayloadType, ScenarioEvent, ScenarioId, TransportBinding, WorkerKind,
};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TelemetryFrame {
    pub schema_version: String,
    pub run_id: String,
    pub scenario_id: ScenarioId,
    pub worker_id: String,
    pub worker_kind: WorkerKind,
    pub sequence: u64,
    pub emitted_at: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub received_at: Option<String>,
    pub payload_type: PayloadType,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stream_quality: Option<StreamQuality>,
    pub payload: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct StreamQuality {
    pub quality: StreamQualityStatus,
    pub source_lag_ms: f64,
    pub collector_lag_ms: f64,
    pub dropped_frame_count: u64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum StreamQualityStatus {
    Good,
    Lagging,
    Degraded,
    Unknown,
}

#[derive(Debug, Clone)]
pub struct GeneratedFrame {
    pub offset_sec: f64,
    pub frame: TelemetryFrame,
}

#[derive(Debug, Clone)]
pub struct GeneratedRun {
    pub frames: Vec<GeneratedFrame>,
    pub worker_stats: Vec<GeneratedWorkerStats>,
    pub requested_frame_count: usize,
    pub dropped_frame_count: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SimulatedResultFrame {
    pub schema_version: String,
    pub run_id: String,
    pub scenario_id: ScenarioId,
    pub worker_id: String,
    pub worker_kind: WorkerKind,
    pub sequence: u64,
    pub produced_at: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub received_at: Option<String>,
    pub result_type: SimulatedResultType,
    pub model_id: String,
    pub input_window: ResultInputWindow,
    pub value_basis: String,
    pub synthetic_status: String,
    pub values: Vec<SimulatedResultValue>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub lineage_inputs: Vec<ResultLineageInput>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum SimulatedResultType {
    SyntheticEngineeringState,
    SyntheticPhysicsState,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResultInputWindow {
    pub start: String,
    pub end: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SimulatedResultValue {
    pub result_id: String,
    pub entity_id: String,
    pub value_id: String,
    pub label: String,
    pub unit: String,
    pub value: Value,
    pub confidence: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResultLineageInput {
    pub source_kind: String,
    pub source_id: String,
    pub value_basis: String,
}

#[derive(Debug, Clone)]
pub struct GeneratedWorkerStats {
    pub worker_id: String,
    pub requested_frames: usize,
    pub emitted_frames: usize,
    pub dropped_frame_count: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RunSummary {
    pub schema_version: String,
    pub run_id: String,
    pub scenario_id: ScenarioId,
    pub completed_at: String,
    pub lifecycle: SummaryLifecycle,
    pub frame_count: usize,
    #[serde(default)]
    pub requested_frame_count: usize,
    #[serde(default)]
    pub dropped_frame_count: u64,
    pub workers: Vec<WorkerSummary>,
    pub aggregate_metrics: Vec<AggregateMetric>,
    pub events: Vec<ScenarioEvent>,
    pub degraded_intervals: Vec<DegradedInterval>,
    pub artifacts: Vec<SummaryArtifact>,
    pub provenance: SummaryProvenance,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SummaryLifecycle {
    Complete,
    Failed,
    Stopped,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkerSummary {
    pub worker_id: String,
    pub worker_kind: WorkerKind,
    pub payload_type: PayloadType,
    pub frames: usize,
    pub dropped_frame_count: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AggregateMetric {
    pub metric_path: String,
    pub min: f64,
    pub avg: f64,
    pub max: f64,
    pub unit: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DegradedInterval {
    pub start_offset_sec: u32,
    pub end_offset_sec: u32,
    pub reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SummaryArtifact {
    pub artifact_id: String,
    pub path: String,
    pub media_type: String,
    pub description: String,
}

impl From<&ManifestArtifact> for SummaryArtifact {
    fn from(value: &ManifestArtifact) -> Self {
        Self {
            artifact_id: value.artifact_id.clone(),
            path: value.path.clone(),
            media_type: value.media_type.clone(),
            description: format!("Generated artifact reference {}", value.artifact_id),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SummaryProvenance {
    pub generated_by: String,
    pub data_class: String,
    pub storage_format: String,
    pub transport_binding: TransportBinding,
}
