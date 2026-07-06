pub mod sources;

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
