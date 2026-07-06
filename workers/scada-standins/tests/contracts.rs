use std::collections::HashSet;

use scada_standins::{
    SignalKind, ValueBasis, default_tags, resident_source_declaration, telemetry_frames,
};

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

#[test]
fn resident_source_declaration_targets_scada_topic() {
    let source = resident_source_declaration("SRC-MIXED-STANDIN-001");

    assert_eq!(source.schema_version, "scada.resident-source-declaration.v1");
    assert_eq!(source.lifecycle, "resident");
    assert_eq!(source.synthetic_status, "public-safe-standin");
    assert_eq!(source.ingest.topic, "scada.telemetry.v1");
    assert_eq!(source.tags.len(), default_tags().len());
    assert!(source.tags.iter().all(|tag| tag.value_basis == "measured"));
}

#[test]
fn telemetry_frames_are_measured_and_sequence_bound() {
    let frames = telemetry_frames("SRC-MIXED-STANDIN-001", 7);

    assert_eq!(frames.len(), default_tags().len());
    for frame in frames {
        assert_eq!(frame.schema_version, "scada.telemetry.v1");
        assert_eq!(frame.sequence, 7);
        assert_eq!(frame.value_basis, "measured");
        assert_eq!(frame.synthetic_status, "public-safe-standin");
        assert!(!frame.value.as_object().expect("object value").is_empty());
    }
}
