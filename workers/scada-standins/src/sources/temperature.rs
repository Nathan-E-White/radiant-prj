use crate::{SignalKind, SourceTag, ValueBasis};

pub fn tag() -> SourceTag {
    SourceTag {
        tag_id: "TAG-TEMP-LOOP-A",
        asset_id: "ASSET-THERMAL-LOOP-A",
        signal_kind: SignalKind::Temperature,
        unit: "degC",
        value_basis: ValueBasis::Measured,
    }
}
