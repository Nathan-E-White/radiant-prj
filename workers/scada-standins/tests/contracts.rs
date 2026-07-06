use std::collections::HashSet;

use scada_standins::{SignalKind, ValueBasis, default_tags};

#[test]
fn default_tags_cover_mixed_source_set() {
    let kinds: HashSet<_> = default_tags()
        .into_iter()
        .map(|tag| tag.signal_kind)
        .collect();

    for expected in [
        SignalKind::Flux,
        SignalKind::Temperature,
        SignalKind::Pressure,
        SignalKind::ActuatorState,
        SignalKind::ElectricalState,
        SignalKind::Comms,
    ] {
        assert!(kinds.contains(&expected), "missing {expected:?}");
    }
}

#[test]
fn resident_source_tags_are_measured() {
    for tag in default_tags() {
        assert_eq!(tag.value_basis, ValueBasis::Measured);
        assert_eq!(tag.value_basis.as_str(), "measured");
    }
}
