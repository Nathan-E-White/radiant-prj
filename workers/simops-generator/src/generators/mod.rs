pub mod burst;
pub mod fabric;
pub mod scheduler;
pub mod storage;

use serde_json::Value;
use time::format_description::well_known::Rfc3339;
use time::{Duration, OffsetDateTime};

use crate::envelope::{
    GeneratedFrame, GeneratedRun, GeneratedWorkerStats, StreamQuality, StreamQualityStatus,
    TelemetryFrame,
};
use crate::manifest::{PayloadType, RunManifest, WorkerDeclaration, WorkerKind};
use crate::sampler::{SampleOutcome, ScenarioSampler};
use crate::{Result, SimopsError};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WorkerSelection {
    All,
    One(WorkerKind),
}

impl WorkerSelection {
    pub fn parse(value: &str) -> Result<Self> {
        match value {
            "all" => Ok(Self::All),
            "scheduler" => Ok(Self::One(WorkerKind::Scheduler)),
            "storage" => Ok(Self::One(WorkerKind::Storage)),
            "burst" => Ok(Self::One(WorkerKind::Burst)),
            "fabric" => Ok(Self::One(WorkerKind::Fabric)),
            other => Err(SimopsError::new(format!(
                "unknown worker {other}; expected scheduler, storage, burst, fabric, or all"
            ))),
        }
    }

    fn includes(self, worker_kind: WorkerKind) -> bool {
        match self {
            Self::All => true,
            Self::One(selected) => selected == worker_kind,
        }
    }
}

pub struct FrameContext<'a> {
    pub manifest: &'a RunManifest,
    pub worker: &'a WorkerDeclaration,
    pub sequence: u64,
    pub offset_sec: f64,
    pub base_pressure: f64,
    pub pressure: f64,
}

pub fn generate_run(
    manifest: &RunManifest,
    selection: WorkerSelection,
    frames_override: Option<usize>,
) -> Result<GeneratedRun> {
    let mut frames = Vec::new();
    let mut worker_stats = Vec::new();
    let mut requested_frame_count = 0usize;
    let mut dropped_frame_count = 0u64;
    let mut sampler = ScenarioSampler::new(manifest);

    let selected_workers: Vec<&WorkerDeclaration> = manifest
        .workers
        .iter()
        .filter(|worker| selection.includes(worker.worker_kind))
        .collect();

    if selected_workers.is_empty() {
        return Err(SimopsError::new(
            "worker selection did not match any manifest worker",
        ));
    }

    for worker in selected_workers {
        let frame_count = frames_override.unwrap_or_else(|| inferred_frame_count(manifest, worker));
        requested_frame_count += frame_count;
        let mut emitted_frames = 0usize;
        let mut worker_dropped_frames = 0u64;
        let mut pending_drops = 0u64;

        for index in 0..frame_count {
            let sequence = u64::try_from(index + 1)
                .map_err(|_| SimopsError::new("frame sequence exceeded u64 range"))?;
            let offset_sec = offset_for_frame(index, frame_count, manifest.runtime_limit_sec);
            let base_pressure = manifest.randomization.pressure_at(offset_sec);
            let sample = sampler.sample(worker, sequence, offset_sec, base_pressure)?;

            if !sample.emitted {
                pending_drops += 1;
                worker_dropped_frames += 1;
                dropped_frame_count += 1;
                continue;
            }

            let context = FrameContext {
                manifest,
                worker,
                sequence,
                offset_sec,
                base_pressure,
                pressure: sample.pressure,
            };
            let payload = payload_for(&context)?;
            let emitted_at = timestamp_at(&manifest.created_at, offset_sec, 0.0)?;
            let received_at = sample
                .received_lag_ms
                .map(|lag_ms| timestamp_at(&manifest.created_at, offset_sec, lag_ms))
                .transpose()?;
            let frame = TelemetryFrame {
                schema_version: "simops.telemetry.v1".to_string(),
                run_id: manifest.run_id.clone(),
                scenario_id: manifest.scenario_id,
                worker_id: worker.worker_id.clone(),
                worker_kind: worker.worker_kind,
                sequence,
                emitted_at,
                received_at,
                payload_type: worker.payload_type,
                stream_quality: Some(stream_quality(&sample, pending_drops)),
                payload,
            };
            frames.push(GeneratedFrame { offset_sec, frame });
            emitted_frames += 1;
            pending_drops = 0;
        }

        worker_stats.push(GeneratedWorkerStats {
            worker_id: worker.worker_id.clone(),
            requested_frames: frame_count,
            emitted_frames,
            dropped_frame_count: worker_dropped_frames,
        });
    }

    frames.sort_by(|left, right| {
        left.offset_sec
            .partial_cmp(&right.offset_sec)
            .unwrap_or(std::cmp::Ordering::Equal)
            .then(left.frame.worker_id.cmp(&right.frame.worker_id))
    });

    Ok(GeneratedRun {
        frames,
        worker_stats,
        requested_frame_count,
        dropped_frame_count,
    })
}

fn payload_for(context: &FrameContext<'_>) -> Result<Value> {
    match context.worker.worker_kind {
        WorkerKind::Scheduler => scheduler::generate(context),
        WorkerKind::Storage => storage::generate(context),
        WorkerKind::Burst => burst::generate(context),
        WorkerKind::Fabric => fabric::generate(context),
    }
}

fn inferred_frame_count(manifest: &RunManifest, worker: &WorkerDeclaration) -> usize {
    let count = (f64::from(manifest.runtime_limit_sec) * worker.emit_hz).ceil() as usize;
    count.clamp(1, 1_000_000)
}

fn offset_for_frame(index: usize, frame_count: usize, runtime_limit_sec: u32) -> f64 {
    if frame_count <= 1 {
        return 0.0;
    }

    let progress = index as f64 / (frame_count - 1) as f64;
    progress * f64::from(runtime_limit_sec)
}

fn timestamp_at(created_at: &str, offset_sec: f64, lag_ms: f64) -> Result<String> {
    let base = OffsetDateTime::parse(created_at, &Rfc3339)?;
    let shifted =
        base + Duration::milliseconds((offset_sec * 1000.0 + lag_ms.max(0.0)).round() as i64);
    Ok(shifted.format(&Rfc3339)?)
}

fn stream_quality(sample: &SampleOutcome, dropped_frame_count: u64) -> StreamQuality {
    let pressure = sample.pressure;
    let quality = if dropped_frame_count > 0 || sample.burst_bad || pressure >= 0.75 {
        StreamQualityStatus::Degraded
    } else if pressure >= 0.55 {
        StreamQualityStatus::Lagging
    } else {
        StreamQualityStatus::Good
    };

    StreamQuality {
        quality,
        source_lag_ms: round1(sample.source_lag_ms),
        collector_lag_ms: round1(sample.collector_lag_ms),
        dropped_frame_count,
    }
}

pub fn ensure_payload_matches(worker_kind: WorkerKind, payload_type: PayloadType) -> Result<()> {
    let expected = worker_kind.expected_payload_type();
    if payload_type != expected {
        return Err(SimopsError::new(format!(
            "worker kind {} expected payload {:?}, got {:?}",
            worker_kind.as_str(),
            expected,
            payload_type
        )));
    }
    Ok(())
}

pub fn clamp(value: f64, min: f64, max: f64) -> f64 {
    value.max(min).min(max)
}

pub fn round1(value: f64) -> f64 {
    (value * 10.0).round() / 10.0
}

pub fn round3(value: f64) -> f64 {
    (value * 1000.0).round() / 1000.0
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::manifest::{PayloadType, WorkerKind};

    #[test]
    fn clamps_values_to_bounds() {
        assert_eq!(clamp(-1.0, 0.0, 10.0), 0.0);
        assert_eq!(clamp(12.0, 0.0, 10.0), 10.0);
        assert_eq!(clamp(5.0, 0.0, 10.0), 5.0);
    }

    #[test]
    fn rejects_wrong_payload_mapping() {
        assert!(
            ensure_payload_matches(WorkerKind::Scheduler, PayloadType::CheckpointStorage).is_err()
        );
        assert!(ensure_payload_matches(WorkerKind::Fabric, PayloadType::FabricProfiler).is_ok());
    }

    #[test]
    fn preserves_monotonic_sequences_when_logical_frames_drop() {
        let manifest = serde_json::json!({
            "schemaVersion": "simops.run-manifest.v1",
            "runId": "RUN-HPC-DROPS-001",
            "scenarioId": "scheduler-drift",
            "createdAt": "2026-07-04T18:00:00.000Z",
            "lifecycle": "created",
            "workbenchAnchor": {
                "jobId": "JOB-HPC-DROPS",
                "evidencePackId": "EP-HPC-DROPS",
                "gatewayEvidenceId": "SLURM-GATEWAY-DROPS"
            },
            "runtimeLimitSec": 20,
            "transportBinding": "ndjson",
            "randomization": {
                "mode": "correlated-pressure",
                "randomSeed": 40401,
                "pressureCurve": [
                    { "offsetSec": 0, "pressure": 0.0 },
                    { "offsetSec": 10, "pressure": 1.0 },
                    { "offsetSec": 20, "pressure": 0.0 }
                ],
                "baseline": [
                    { "metricPath": "scheduler.barrierWaitMs.p95", "nominalMin": 8, "nominalMax": 35, "unit": "ms" }
                ],
                "couplings": [],
                "events": [],
                "bounds": [
                    { "metricPath": "payload.barrierWaitMs.p95", "min": 0, "max": 1000, "unit": "ms" }
                ],
                "distributionProfile": {
                    "emission": {
                        "probability": { "base": 1.0, "pressureSlope": -1.0, "min": 0.0, "max": 1.0 }
                    }
                }
            },
            "workers": [
                {
                    "workerId": "scheduler-01",
                    "workerKind": "scheduler",
                    "payloadType": "schedulerCoScheduling",
                    "emitHz": 1,
                    "panelTarget": "Scheduler"
                }
            ],
            "artifacts": [
                {
                    "artifactId": "simops-run-manifest.drops",
                    "path": "examples/simulation-ops/run-manifest.scheduler-drift.json",
                    "mediaType": "application/json"
                }
            ],
            "provenance": {
                "generatedBy": "simops-generator-test",
                "dataClass": "synthetic-simulation-ops",
                "storageFormat": "ndjson"
            }
        });
        let manifest = RunManifest::from_json(&manifest.to_string()).expect("valid manifest");
        let run = generate_run(&manifest, WorkerSelection::All, Some(3)).expect("generated run");
        let sequences: Vec<u64> = run
            .frames
            .iter()
            .map(|generated| generated.frame.sequence)
            .collect();
        let dropped_in_next_frame = run.frames[1]
            .frame
            .stream_quality
            .as_ref()
            .expect("stream quality")
            .dropped_frame_count;

        assert_eq!(sequences, vec![1, 3]);
        assert_eq!(run.requested_frame_count, 3);
        assert_eq!(run.dropped_frame_count, 1);
        assert_eq!(dropped_in_next_frame, 1);
    }
}
