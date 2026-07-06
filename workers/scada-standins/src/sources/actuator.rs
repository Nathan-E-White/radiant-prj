use crate::{SignalKind, SourceTag, ValueBasis};

pub fn tag() -> SourceTag {
    SourceTag {
        tag_id: "TAG-ACTUATOR-VALVE-A",
        asset_id: "ASSET-ACTUATOR-A",
        signal_kind: SignalKind::ActuatorState,
        unit: "state",
        value_basis: ValueBasis::Measured,
    }
}
