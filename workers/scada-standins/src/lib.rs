pub mod sources;

use serde::Serialize;
use serde_json::{Value, json};

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ValueBasis {
    Measured,
    Imputed,
    Simulated,
}

impl ValueBasis {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Measured => "measured",
            Self::Imputed => "imputed",
            Self::Simulated => "simulated",
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum SignalKind {
    Flux,
    Temperature,
    Pressure,
    ActuatorState,
    ElectricalState,
    Comms,
}

impl SignalKind {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Flux => "flux",
            Self::Temperature => "temperature",
            Self::Pressure => "pressure",
            Self::ActuatorState => "actuatorState",
            Self::ElectricalState => "electricalState",
            Self::Comms => "comms",
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SourceTag {
    pub tag_id: &'static str,
    pub asset_id: &'static str,
    pub signal_kind: SignalKind,
    pub unit: &'static str,
    pub value_basis: ValueBasis,
}

pub fn default_tags() -> Vec<SourceTag> {
    sources::all_tags()
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ResidentSourceDeclaration {
    pub schema_version: &'static str,
    pub source_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reactor_id: Option<String>,
    pub display_name: &'static str,
    pub lifecycle: &'static str,
    pub synthetic_status: &'static str,
    pub ingest: ScadaIngest,
    pub tags: Vec<ResidentSourceTag>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ScadaIngest {
    pub topic: &'static str,
    pub endpoint_kind: &'static str,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ResidentSourceTag {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reactor_id: Option<String>,
    pub tag_id: String,
    pub asset_id: String,
    pub signal_kind: &'static str,
    pub unit: &'static str,
    pub value_basis: &'static str,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct TelemetryFrame {
    pub schema_version: &'static str,
    pub source_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reactor_id: Option<String>,
    pub tag_id: String,
    pub asset_id: String,
    pub signal_kind: &'static str,
    pub sampled_at: String,
    pub observed_at: String,
    pub sequence: u64,
    pub unit: &'static str,
    pub value: Value,
    pub quality: &'static str,
    pub value_basis: &'static str,
    pub synthetic_status: &'static str,
}

pub fn resident_source_declaration(source_id: &str) -> ResidentSourceDeclaration {
    ResidentSourceDeclaration {
        schema_version: "scada.resident-source-declaration.v1",
        source_id: source_id.to_string(),
        reactor_id: None,
        display_name: "Mixed public-safe resident source stand-ins",
        lifecycle: "resident",
        synthetic_status: "public-safe-standin",
        ingest: ScadaIngest {
            topic: "scada.telemetry.v1",
            endpoint_kind: "gateway-http",
        },
        tags: default_tags()
            .into_iter()
            .map(|tag| ResidentSourceTag {
                source_id: None,
                reactor_id: None,
                tag_id: tag.tag_id.to_string(),
                asset_id: tag.asset_id.to_string(),
                signal_kind: tag.signal_kind.as_str(),
                unit: tag.unit,
                value_basis: tag.value_basis.as_str(),
            })
            .collect(),
    }
}

pub fn telemetry_frames(source_id: &str, sequence: u64) -> Vec<TelemetryFrame> {
    let sampled_at = timestamp_for(sequence, 0);
    let observed_at = timestamp_for(sequence, 1);
    default_tags()
        .into_iter()
        .map(|tag| TelemetryFrame {
            schema_version: "scada.telemetry.v1",
            source_id: source_id.to_string(),
            reactor_id: None,
            tag_id: tag.tag_id.to_string(),
            asset_id: tag.asset_id.to_string(),
            signal_kind: tag.signal_kind.as_str(),
            sampled_at: sampled_at.clone(),
            observed_at: observed_at.clone(),
            sequence,
            unit: tag.unit,
            value: measured_value(tag.signal_kind, sequence),
            quality: if tag.signal_kind == SignalKind::Comms {
                "stale"
            } else {
                "good"
            },
            value_basis: tag.value_basis.as_str(),
            synthetic_status: "public-safe-standin",
        })
        .collect()
}

pub fn reactor_resident_source_declaration(
    source_id: &str,
    reactor_id: &str,
    worker_index: usize,
) -> ResidentSourceDeclaration {
    ResidentSourceDeclaration {
        schema_version: "scada.resident-source-declaration.v1",
        source_id: source_id.to_string(),
        reactor_id: Some(reactor_id.to_string()),
        display_name: "Reactor-scoped public-safe resident source stand-in",
        lifecycle: "resident",
        synthetic_status: "public-safe-standin",
        ingest: ScadaIngest {
            topic: "scada.telemetry.v1",
            endpoint_kind: "gateway-http",
        },
        tags: reactor_tags(source_id, reactor_id, worker_index)
            .into_iter()
            .map(|(tag, tag_id, asset_id)| ResidentSourceTag {
                source_id: Some(source_id.to_string()),
                reactor_id: Some(reactor_id.to_string()),
                tag_id,
                asset_id,
                signal_kind: tag.signal_kind.as_str(),
                unit: tag.unit,
                value_basis: tag.value_basis.as_str(),
            })
            .collect(),
    }
}

pub fn reactor_telemetry_frames(
    source_id: &str,
    reactor_id: &str,
    worker_index: usize,
    sequence: u64,
) -> Vec<TelemetryFrame> {
    let sampled_at = timestamp_for(sequence, 0);
    let observed_at = timestamp_for(sequence, 1);
    reactor_tags(source_id, reactor_id, worker_index)
        .into_iter()
        .map(|(tag, tag_id, asset_id)| TelemetryFrame {
            schema_version: "scada.telemetry.v1",
            source_id: source_id.to_string(),
            reactor_id: Some(reactor_id.to_string()),
            tag_id,
            asset_id,
            signal_kind: tag.signal_kind.as_str(),
            sampled_at: sampled_at.clone(),
            observed_at: observed_at.clone(),
            sequence,
            unit: tag.unit,
            value: measured_value(tag.signal_kind, sequence),
            quality: if tag.signal_kind == SignalKind::Comms {
                "stale"
            } else {
                "good"
            },
            value_basis: tag.value_basis.as_str(),
            synthetic_status: "public-safe-standin",
        })
        .collect()
}

fn reactor_tags(
    source_id: &str,
    reactor_id: &str,
    worker_index: usize,
) -> Vec<(SourceTag, String, String)> {
    if worker_index >= 3 {
        return Vec::new();
    }
    default_tags()
        .into_iter()
        .skip(worker_index * 2)
        .take(2)
        .map(|tag| {
            let tag_id = format!("{}-{}", source_id, tag.signal_kind.as_str());
            let asset_id = format!("{}-unit", reactor_id);
            (tag, tag_id, asset_id)
        })
        .collect()
}

fn timestamp_for(sequence: u64, lag_sec: u64) -> String {
    let second = (sequence + lag_sec).min(59);
    format!("2026-07-06T15:00:{second:02}Z")
}

fn measured_value(kind: SignalKind, sequence: u64) -> Value {
    let step = sequence.saturating_sub(1) as f64;
    match kind {
        SignalKind::Flux => json!({ "scalar": round3(0.82 + step * 0.002) }),
        SignalKind::Temperature => json!({ "scalar": round1(612.4 + step * 0.4) }),
        SignalKind::Pressure => json!({ "scalar": round1(15.2 + step * 0.02) }),
        SignalKind::ActuatorState => json!({ "state": "position-hold", "positionPct": 63 }),
        SignalKind::ElectricalState => json!({ "voltageKv": 13.8, "breakerClosed": true }),
        SignalKind::Comms => {
            json!({ "latencyMs": round1(18.4 + step * 0.3), "packetLossPct": 0.2 })
        }
    }
}

fn round1(value: f64) -> f64 {
    (value * 10.0).round() / 10.0
}

fn round3(value: f64) -> f64 {
    (value * 1000.0).round() / 1000.0
}
