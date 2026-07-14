use std::collections::HashSet;

use scada_standins::{
    SignalKind, ValueBasis, default_tags, reactor_resident_source_declaration,
    reactor_telemetry_frames, resident_source_declaration, telemetry_frames,
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
fn reactor_worker_identity_scopes_declaration_and_frames() {
    let declaration = reactor_resident_source_declaration("src-stable-01", "reactor-opaque-a", 1);
    assert_eq!(declaration.source_id, "src-stable-01");
    assert_eq!(declaration.reactor_id.as_deref(), Some("reactor-opaque-a"));
    assert_eq!(declaration.tags.len(), 2);
    assert!(declaration.tags.iter().all(|tag| {
        tag.source_id.as_deref() == Some("src-stable-01")
            && tag.reactor_id.as_deref() == Some("reactor-opaque-a")
            && tag.tag_id.starts_with("src-stable-01-")
    }));

    let frames = reactor_telemetry_frames("src-stable-01", "reactor-opaque-a", 1, 7);
    assert_eq!(frames.len(), 2);
    assert!(frames.iter().all(|frame| {
        frame.source_id == "src-stable-01"
            && frame.reactor_id.as_deref() == Some("reactor-opaque-a")
            && frame.sequence == 7
            && frame.value_basis == "measured"
    }));
}

#[test]
fn reactor_worker_index_cannot_expand_beyond_three_sources() {
    assert!(
        reactor_resident_source_declaration("src", "reactor", 3)
            .tags
            .is_empty()
    );
    assert!(reactor_telemetry_frames("src", "reactor", 3, 1).is_empty());
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

    assert_eq!(
        source.schema_version,
        "scada.resident-source-declaration.v1"
    );
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
