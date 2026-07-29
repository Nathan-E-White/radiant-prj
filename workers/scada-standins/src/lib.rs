mod sources;

use serde::Serialize;
use serde_json::{Value, json};

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

const MAX_REACTOR_WORKERS: usize = 3;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ValueBasis {
    Measured,
}

impl ValueBasis {
    pub fn as_str(self) -> &'static str {
        "measured"
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
enum SignalKind {
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
struct SourceTag {
    pub tag_id: &'static str,
    pub asset_id: &'static str,
    pub signal_kind: SignalKind,
    pub unit: &'static str,
    pub value_basis: ValueBasis,
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

#[derive(Debug, Clone)]
struct ResolvedTag {
    tag_id: String,
    asset_id: String,
    signal_kind: SignalKind,
    unit: &'static str,
    value_basis: ValueBasis,
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

pub trait Clock {
    fn now(&mut self) -> Result<String>;
}
pub trait Cadence {
    fn wait_for_next_batch(&mut self) -> Result<bool>;
}
pub trait ResidentSourcePublisher {
    fn register(&mut self, declaration: &ResidentSourceDeclaration) -> Result<()>;
    fn publish(&mut self, frames: &[TelemetryFrame]) -> Result<()>;
}

#[derive(Debug, Clone)]
enum ResidentSourceConfiguration {
    Static {
        source_id: String,
        max_frames: Option<u64>,
    },
    ReactorScoped {
        source_id: String,
        reactor_id: String,
        worker_index: usize,
        max_frames: u64,
    },
}

#[derive(Debug, Clone)]
pub struct ResidentSource {
    configuration: ResidentSourceConfiguration,
    next_sequence: u64,
}

impl ResidentSource {
    pub fn static_source(source_id: impl Into<String>) -> Self {
        Self {
            configuration: ResidentSourceConfiguration::Static {
                source_id: source_id.into(),
                max_frames: None,
            },
            next_sequence: 1,
        }
    }

    pub fn static_source_bounded(source_id: impl Into<String>, max_frames: u64) -> Result<Self> {
        if max_frames == 0 {
            return Err("max frames must be greater than zero".into());
        }
        Ok(Self {
            configuration: ResidentSourceConfiguration::Static {
                source_id: source_id.into(),
                max_frames: Some(max_frames),
            },
            next_sequence: 1,
        })
    }

    pub fn reactor_scoped(
        source_id: impl Into<String>,
        reactor_id: impl Into<String>,
        worker_index: usize,
        max_frames: u64,
    ) -> Result<Self> {
        if worker_index >= MAX_REACTOR_WORKERS {
            return Err("worker index must be 0, 1, or 2".into());
        }
        if max_frames == 0 {
            return Err("max frames must be greater than zero".into());
        }
        Ok(Self {
            configuration: ResidentSourceConfiguration::ReactorScoped {
                source_id: source_id.into(),
                reactor_id: reactor_id.into(),
                worker_index,
                max_frames,
            },
            next_sequence: 1,
        })
    }

    pub fn run<P: ResidentSourcePublisher, C: Clock, D: Cadence>(
        &mut self,
        publisher: &mut P,
        clock: &mut C,
        cadence: &mut D,
    ) -> Result<()> {
        publisher.register(&self.declaration())?;
        loop {
            publisher.publish(&self.next_frames(clock)?)?;
            if self.is_complete() || !cadence.wait_for_next_batch()? {
                return Ok(());
            }
        }
    }

    fn declaration(&self) -> ResidentSourceDeclaration {
        let definition = self.definition();
        ResidentSourceDeclaration {
            schema_version: "scada.resident-source-declaration.v1",
            source_id: definition.source_id,
            reactor_id: definition.reactor_id,
            display_name: definition.display_name,
            lifecycle: "resident",
            synthetic_status: "public-safe-standin",
            ingest: ScadaIngest {
                topic: "scada.telemetry.v1",
                endpoint_kind: "gateway-http",
            },
            tags: definition
                .tags
                .into_iter()
                .map(|tag| ResidentSourceTag {
                    source_id: self.source_id(),
                    reactor_id: self.reactor_id(),
                    tag_id: tag.tag_id,
                    asset_id: tag.asset_id,
                    signal_kind: tag.signal_kind.as_str(),
                    unit: tag.unit,
                    value_basis: tag.value_basis.as_str(),
                })
                .collect(),
        }
    }

    fn next_frames<C: Clock>(&mut self, clock: &mut C) -> Result<Vec<TelemetryFrame>> {
        let observed_at = clock.now()?;
        let sequence = self.next_sequence;
        self.next_sequence += 1;
        let definition = self.definition();
        Ok(definition
            .tags
            .into_iter()
            .map(|tag| TelemetryFrame {
                schema_version: "scada.telemetry.v1",
                source_id: definition.source_id.clone(),
                reactor_id: definition.reactor_id.clone(),
                tag_id: tag.tag_id,
                asset_id: tag.asset_id,
                signal_kind: tag.signal_kind.as_str(),
                sampled_at: observed_at.clone(),
                observed_at: observed_at.clone(),
                sequence,
                unit: tag.unit,
                value: measured_value(tag.signal_kind, sequence),
                quality: if tag.signal_kind == SignalKind::Comms {
                    "stale"
                } else {
                    "good"
                },
                value_basis: "measured",
                synthetic_status: "public-safe-standin",
            })
            .collect())
    }

    fn is_complete(&self) -> bool {
        matches!(&self.configuration, ResidentSourceConfiguration::Static { max_frames: Some(max_frames), .. } if self.next_sequence > *max_frames)
            || matches!(&self.configuration, ResidentSourceConfiguration::ReactorScoped { max_frames, .. } if self.next_sequence > *max_frames)
    }

    fn definition(&self) -> SourceDefinition {
        match &self.configuration {
            ResidentSourceConfiguration::Static { source_id, .. } => {
                SourceDefinition::static_source(source_id)
            }
            ResidentSourceConfiguration::ReactorScoped {
                source_id,
                reactor_id,
                worker_index,
                ..
            } => SourceDefinition::reactor_scoped(source_id, reactor_id, *worker_index),
        }
    }

    fn source_id(&self) -> Option<String> {
        match &self.configuration {
            ResidentSourceConfiguration::Static { .. } => None,
            ResidentSourceConfiguration::ReactorScoped { source_id, .. } => Some(source_id.clone()),
        }
    }
    fn reactor_id(&self) -> Option<String> {
        match &self.configuration {
            ResidentSourceConfiguration::Static { .. } => None,
            ResidentSourceConfiguration::ReactorScoped { reactor_id, .. } => {
                Some(reactor_id.clone())
            }
        }
    }
}

struct SourceDefinition {
    source_id: String,
    reactor_id: Option<String>,
    display_name: &'static str,
    tags: Vec<ResolvedTag>,
}
impl SourceDefinition {
    fn static_source(source_id: &str) -> Self {
        Self {
            source_id: source_id.to_string(),
            reactor_id: None,
            display_name: "Mixed public-safe resident source stand-ins",
            tags: resolve_tags(None, None, sources::all_tags()),
        }
    }
    fn reactor_scoped(source_id: &str, reactor_id: &str, worker_index: usize) -> Self {
        Self {
            source_id: source_id.to_string(),
            reactor_id: Some(reactor_id.to_string()),
            display_name: "Reactor-scoped public-safe resident source stand-in",
            tags: resolve_tags(
                Some(source_id),
                Some(reactor_id),
                sources::all_tags()
                    .into_iter()
                    .skip(worker_index * 2)
                    .take(2)
                    .collect(),
            ),
        }
    }
}

fn resolve_tags(
    source_id: Option<&str>,
    reactor_id: Option<&str>,
    tags: Vec<SourceTag>,
) -> Vec<ResolvedTag> {
    tags.into_iter()
        .map(|tag| {
            let tag_id = source_id.map_or_else(
                || tag.tag_id.to_string(),
                |id| format!("{}-{}", id, tag.signal_kind.as_str()),
            );
            let asset_id =
                reactor_id.map_or_else(|| tag.asset_id.to_string(), |id| format!("{}-unit", id));
            ResolvedTag {
                tag_id,
                asset_id,
                signal_kind: tag.signal_kind,
                unit: tag.unit,
                value_basis: tag.value_basis,
            }
        })
        .collect()
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
