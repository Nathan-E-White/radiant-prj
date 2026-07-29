use scada_standins::{
    Cadence, Clock, ResidentSource, ResidentSourceDeclaration, ResidentSourcePublisher, Result,
    TelemetryFrame,
};

#[derive(Default)]
struct RecordingPublisher {
    declarations: Vec<ResidentSourceDeclaration>,
    batches: Vec<Vec<TelemetryFrame>>,
    fail_register: bool,
    fail_publish_at: Option<usize>,
}
impl ResidentSourcePublisher for RecordingPublisher {
    fn register(&mut self, declaration: &ResidentSourceDeclaration) -> Result<()> {
        if self.fail_register {
            return Err("registration failed".into());
        }
        self.declarations.push(declaration.clone());
        Ok(())
    }
    fn publish(&mut self, frames: &[TelemetryFrame]) -> Result<()> {
        if self.fail_publish_at == Some(self.batches.len() + 1) {
            return Err("publication failed".into());
        }
        self.batches.push(frames.to_vec());
        Ok(())
    }
}
struct ControlledClock {
    tick: u64,
}
impl Clock for ControlledClock {
    fn now(&mut self) -> Result<String> {
        self.tick += 1;
        Ok(format!(
            "2026-07-06T15:{:02}:{:02}Z",
            1 + self.tick / 60,
            self.tick % 60
        ))
    }
}
struct Batches(usize);
impl Cadence for Batches {
    fn wait_for_next_batch(&mut self) -> Result<bool> {
        if self.0 == 0 {
            Ok(false)
        } else {
            self.0 -= 1;
            Ok(true)
        }
    }
}

#[test]
fn static_source_registers_before_long_running_measured_emission() {
    let mut source = ResidentSource::static_source("SRC-MIXED-STANDIN-001");
    let mut publisher = RecordingPublisher::default();
    let mut clock = ControlledClock { tick: 0 };
    let mut cadence = Batches(60);
    source
        .run(&mut publisher, &mut clock, &mut cadence)
        .expect("emit static source");
    assert_eq!(publisher.declarations.len(), 1);
    assert_eq!(publisher.batches.len(), 61);
    assert!(
        publisher
            .batches
            .iter()
            .flatten()
            .all(|frame| frame.value_basis == "measured")
    );
    assert_eq!(publisher.batches[59][0].sequence, 60);
    assert_eq!(publisher.batches[58][0].sampled_at, "2026-07-06T15:01:59Z");
    assert_eq!(publisher.batches[59][0].sampled_at, "2026-07-06T15:02:00Z");
    assert_eq!(publisher.batches[60][0].sampled_at, "2026-07-06T15:02:01Z");
}

#[test]
fn reactor_scoped_source_preserves_identity_and_worker_cap() {
    let mut source = ResidentSource::reactor_scoped("src-stable-01", "reactor-opaque-a", 1, 2)
        .expect("configure source");
    let mut publisher = RecordingPublisher::default();
    let mut clock = ControlledClock { tick: 0 };
    let mut cadence = Batches(10);
    source
        .run(&mut publisher, &mut clock, &mut cadence)
        .expect("emit reactor source");
    assert_eq!(publisher.declarations[0].tags.len(), 2);
    assert_eq!(publisher.batches.len(), 2);
    assert!(
        publisher
            .batches
            .iter()
            .flatten()
            .all(|frame| frame.source_id == "src-stable-01"
                && frame.reactor_id.as_deref() == Some("reactor-opaque-a")
                && frame.value_basis == "measured")
    );
    assert!(ResidentSource::reactor_scoped("src", "reactor", 3, 1).is_err());
}

#[test]
fn static_source_honors_an_explicit_smoke_test_bound() {
    let mut source = ResidentSource::static_source_bounded("src-static", 1).expect("configure bounded static source");
    let mut publisher = RecordingPublisher::default();
    source.run(&mut publisher, &mut ControlledClock { tick: 0 }, &mut Batches(10)).expect("emit bounded static source");
    assert_eq!(publisher.batches.len(), 1);
}

#[test]
fn registration_failure_prevents_publication() {
    let mut source = ResidentSource::static_source("src");
    let mut publisher = RecordingPublisher {
        fail_register: true,
        ..Default::default()
    };
    assert!(
        source
            .run(
                &mut publisher,
                &mut ControlledClock { tick: 0 },
                &mut Batches(1)
            )
            .is_err()
    );
    assert!(publisher.batches.is_empty());
}

#[test]
fn publication_failure_stops_the_run() {
    let mut source = ResidentSource::static_source("src");
    let mut publisher = RecordingPublisher {
        fail_publish_at: Some(2),
        ..Default::default()
    };
    assert!(
        source
            .run(
                &mut publisher,
                &mut ControlledClock { tick: 0 },
                &mut Batches(3)
            )
            .is_err()
    );
    assert_eq!(publisher.declarations.len(), 1);
    assert_eq!(publisher.batches.len(), 1);
}
