use std::collections::HashMap;
use std::fs::{File, create_dir_all};
use std::io::{self, BufWriter, Write};
use std::path::Path;

use serde_json::Value;
use time::format_description::well_known::Rfc3339;
use time::{Duration, OffsetDateTime};

use crate::envelope::{
    AggregateMetric, DegradedInterval, GeneratedRun, GeneratedWorkerStats, RunSummary,
    StreamQualityStatus, SummaryArtifact, SummaryLifecycle, SummaryProvenance, WorkerSummary,
};
use crate::generators::round1;
use crate::manifest::{MetricBound, RunManifest, WorkerDeclaration};
use crate::{Result, SimopsError};

const AGGREGATE_PATHS: &[&str] = &[
    "payload.barrierWaitMs.p95",
    "payload.syncLatencyUs.p95",
    "payload.nvmeOf.cacheSaturationPct",
    "payload.parallelCluster.cloudReady",
    "payload.nonblockingComm.completionOverheadPct",
];

pub fn write_ndjson(run: &GeneratedRun, path: &str) -> Result<()> {
    let mut writer = writer_for_path(path)?;
    for generated in &run.frames {
        serde_json::to_writer(&mut writer, &generated.frame)?;
        writer.write_all(b"\n")?;
    }
    writer.flush()?;
    Ok(())
}

pub fn write_summary(summary: &RunSummary, path: &str) -> Result<()> {
    let mut writer = writer_for_path(path)?;
    serde_json::to_writer_pretty(&mut writer, summary)?;
    writer.write_all(b"\n")?;
    writer.flush()?;
    Ok(())
}

pub fn build_summary(manifest: &RunManifest, run: &GeneratedRun) -> Result<RunSummary> {
    let workers = worker_summaries(&manifest.workers, run);
    let aggregate_metrics = aggregate_metrics(&manifest.randomization.bounds, run);

    Ok(RunSummary {
        schema_version: "simops.run-summary.v1".to_string(),
        run_id: manifest.run_id.clone(),
        scenario_id: manifest.scenario_id,
        completed_at: completed_at(&manifest.created_at, manifest.runtime_limit_sec)?,
        lifecycle: SummaryLifecycle::Complete,
        frame_count: run.frames.len(),
        requested_frame_count: run.requested_frame_count,
        dropped_frame_count: run.dropped_frame_count,
        workers,
        aggregate_metrics,
        events: manifest.randomization.events.clone(),
        degraded_intervals: degraded_intervals(run, manifest.runtime_limit_sec),
        artifacts: manifest
            .artifacts
            .iter()
            .map(SummaryArtifact::from)
            .collect(),
        provenance: SummaryProvenance {
            generated_by: "simops-generator".to_string(),
            data_class: "synthetic-simulation-ops".to_string(),
            storage_format: "json".to_string(),
            transport_binding: manifest.transport_binding,
        },
    })
}

fn writer_for_path(path: &str) -> Result<Box<dyn Write>> {
    if path == "-" {
        return Ok(Box::new(BufWriter::new(io::stdout())));
    }

    let path = Path::new(path);
    if let Some(parent) = path.parent() {
        if !parent.as_os_str().is_empty() {
            create_dir_all(parent)?;
        }
    }

    Ok(Box::new(BufWriter::new(File::create(path)?)))
}

fn completed_at(created_at: &str, runtime_limit_sec: u32) -> Result<String> {
    let base = OffsetDateTime::parse(created_at, &Rfc3339)?;
    let completed = base + Duration::seconds(i64::from(runtime_limit_sec));
    Ok(completed.format(&Rfc3339)?)
}

fn worker_summaries(workers: &[WorkerDeclaration], run: &GeneratedRun) -> Vec<WorkerSummary> {
    let stats_by_worker: HashMap<&str, &GeneratedWorkerStats> = run
        .worker_stats
        .iter()
        .map(|stats| (stats.worker_id.as_str(), stats))
        .collect();

    workers
        .iter()
        .filter_map(|worker| {
            if let Some(stats) = stats_by_worker.get(worker.worker_id.as_str()) {
                return Some(WorkerSummary {
                    worker_id: worker.worker_id.clone(),
                    worker_kind: worker.worker_kind,
                    payload_type: worker.payload_type,
                    frames: stats.emitted_frames,
                    dropped_frame_count: stats.dropped_frame_count,
                });
            }

            let frames = run
                .frames
                .iter()
                .filter(|generated| generated.frame.worker_id == worker.worker_id)
                .count();

            if frames == 0 {
                return None;
            }

            Some(WorkerSummary {
                worker_id: worker.worker_id.clone(),
                worker_kind: worker.worker_kind,
                payload_type: worker.payload_type,
                frames,
                dropped_frame_count: 0,
            })
        })
        .collect()
}

fn aggregate_metrics(bounds: &[MetricBound], run: &GeneratedRun) -> Vec<AggregateMetric> {
    let units_by_path: HashMap<&str, &str> = bounds
        .iter()
        .map(|bound| (bound.metric_path.as_str(), bound.unit.as_str()))
        .collect();

    AGGREGATE_PATHS
        .iter()
        .filter_map(|path| {
            let values: Vec<f64> = run
                .frames
                .iter()
                .filter_map(|generated| numeric_path(&generated.frame.payload, path))
                .collect();

            if values.is_empty() {
                return None;
            }

            let min = values.iter().copied().fold(f64::INFINITY, f64::min);
            let max = values.iter().copied().fold(f64::NEG_INFINITY, f64::max);
            let avg = values.iter().sum::<f64>() / values.len() as f64;
            let unit = units_by_path.get(path).copied().unwrap_or("value");

            Some(AggregateMetric {
                metric_path: (*path).to_string(),
                min: round1(min),
                avg: round1(avg),
                max: round1(max),
                unit: unit.to_string(),
            })
        })
        .collect()
}

fn degraded_intervals(run: &GeneratedRun, runtime_limit_sec: u32) -> Vec<DegradedInterval> {
    let degraded_offsets: Vec<f64> = run
        .frames
        .iter()
        .filter(|generated| {
            generated
                .frame
                .stream_quality
                .as_ref()
                .map(|quality| quality.quality != StreamQualityStatus::Good)
                .unwrap_or(false)
        })
        .map(|generated| generated.offset_sec)
        .collect();

    if degraded_offsets.is_empty() {
        return vec![];
    }

    let start = degraded_offsets
        .iter()
        .copied()
        .fold(f64::INFINITY, f64::min)
        .floor() as u32;
    let end = degraded_offsets
        .iter()
        .copied()
        .fold(f64::NEG_INFINITY, f64::max)
        .ceil() as u32;

    vec![DegradedInterval {
        start_offset_sec: start.min(runtime_limit_sec),
        end_offset_sec: end.min(runtime_limit_sec),
        reason: "correlated scenario pressure produced lagging or degraded stream quality"
            .to_string(),
    }]
}

fn numeric_path(payload: &Value, dotted_path: &str) -> Option<f64> {
    let mut current = payload;
    for segment in dotted_path
        .split('.')
        .skip_while(|segment| *segment == "payload")
    {
        current = current.get(segment)?;
    }
    current.as_f64()
}

pub fn ensure_positive_frames(frames: usize) -> Result<usize> {
    if frames == 0 {
        return Err(SimopsError::new("--frames must be greater than zero"));
    }
    Ok(frames)
}
