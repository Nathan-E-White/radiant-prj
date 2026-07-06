use crate::{SignalKind, SourceTag, ValueBasis};

pub fn tag() -> SourceTag {
    SourceTag {
        tag_id: "TAG-ELECTRICAL-BUS-A",
        asset_id: "ASSET-POWER-BUS-A",
        signal_kind: SignalKind::ElectricalState,
        unit: "kV",
        value_basis: ValueBasis::Measured,
    }
}
