use crate::{SignalKind, SourceTag, ValueBasis};

pub fn tag() -> SourceTag {
    SourceTag {
        tag_id: "TAG-COMMS-LINK-A",
        asset_id: "ASSET-COMMS-LINK-A",
        signal_kind: SignalKind::Comms,
        unit: "ms",
        value_basis: ValueBasis::Measured,
    }
}
