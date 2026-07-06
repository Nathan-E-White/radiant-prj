use crate::{SignalKind, SourceTag, ValueBasis};

pub fn tag() -> SourceTag {
    SourceTag {
        tag_id: "TAG-FLUX-CORE-A",
        asset_id: "ASSET-CORE-A",
        signal_kind: SignalKind::Flux,
        unit: "relative-flux",
        value_basis: ValueBasis::Measured,
    }
}
