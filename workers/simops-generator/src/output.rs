use std::collections::HashMap;
use std::fs::{File, create_dir_all};
use std::io::{self, BufWriter, Read, Write};
use std::net::TcpStream;
use std::path::Path;
use std::time::Duration as StdDuration;

use serde_json::Value;
use serde_json::json;
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

pub fn post_ingest(run: &GeneratedRun, url: &str, token: &str) -> Result<()> {
    let target = HttpTarget::parse(url)?;
    let frames: Vec<_> = run
        .frames
        .iter()
        .map(|generated| &generated.frame)
        .collect();
    let payload = serde_json::to_vec(&json!({ "frames": frames }))?;
    let mut stream = TcpStream::connect(&target.connect_addr)?;
    stream.set_read_timeout(Some(StdDuration::from_secs(10)))?;
    stream.set_write_timeout(Some(StdDuration::from_secs(10)))?;

    let request = format!(
        "POST {} HTTP/1.1\r\nHost: {}\r\nContent-Type: application/json\r\nAccept: application/json\r\nX-Simops-Ingest-Token: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        target.path,
        target.host_header,
        token,
        payload.len()
    );
    stream.write_all(request.as_bytes())?;
    stream.write_all(&payload)?;
    stream.flush()?;

    let mut response = String::new();
    stream.read_to_string(&mut response)?;
    let status_line = response.lines().next().unwrap_or_default();
    if !status_line.starts_with("HTTP/1.1 2") && !status_line.starts_with("HTTP/1.0 2") {
        return Err(SimopsError::new(format!(
            "ingest endpoint returned non-success status: {}",
            status_line
        )));
    }
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

struct HttpTarget {
    connect_addr: String,
    host_header: String,
    path: String,
}

impl HttpTarget {
    fn parse(url: &str) -> Result<Self> {
        let without_scheme = url
            .strip_prefix("http://")
            .ok_or_else(|| SimopsError::new("--ingest-url must use http:// for local ingestion"))?;
        let (host, path) = match without_scheme.split_once('/') {
            Some((host, path)) => (host, format!("/{path}")),
            None => (without_scheme, "/".to_string()),
        };
        if host.is_empty() {
            return Err(SimopsError::new("--ingest-url host is required"));
        }
        let connect_addr = if host.contains(':') {
            host.to_string()
        } else {
            format!("{host}:80")
        };
        Ok(Self {
            connect_addr,
            host_header: host.to_string(),
            path,
        })
    }
}

#[cfg(test)]
mod ingest_tests {
    use super::*;
    use crate::generators::{WorkerSelection, generate_run};
    use crate::manifest::RunManifest;
    use std::io::{Read, Write};
    use std::net::{TcpListener, TcpStream};
    use std::sync::mpsc;
    use std::thread;

    fn manifest() -> RunManifest {
        let raw = serde_json::json!({
            "schemaVersion": "simops.run-manifest.v1",
            "runId": "RUN-TEST-001",
            "scenarioId": "scheduler-drift",
            "createdAt": "2026-07-04T18:00:00.000Z",
            "lifecycle": "created",
            "workbenchAnchor": {
                "jobId": "JOB-HPC-404",
                "evidencePackId": "EP-HPC-404",
                "gatewayEvidenceId": "SLURM-GATEWAY-001"
            },
            "runtimeLimitSec": 10,
            "transportBinding": "ndjson",
            "randomization": {
                "mode": "correlated-pressure",
                "pressureCurve": [
                    { "offsetSec": 0, "pressure": 0.1 },
                    { "offsetSec": 10, "pressure": 0.2 }
                ],
                "baseline": [
                    { "metricPath": "scheduler.barrierWaitMs.p95", "nominalMin": 8, "nominalMax": 35, "unit": "ms" }
                ],
                "couplings": [],
                "events": [],
                "bounds": [
                    { "metricPath": "payload.barrierWaitMs.p95", "min": 0, "max": 1000, "unit": "ms" }
                ]
            },
            "workers": [
                {
                    "workerId": "scheduler-01",
                    "workerKind": "scheduler",
                    "payloadType": "schedulerCoScheduling",
                    "emitHz": 1,
                    "panelTarget": "Multiphysics Job Co-scheduler"
                }
            ],
            "artifacts": [],
            "provenance": {
                "generatedBy": "test",
                "dataClass": "synthetic-simulation-ops",
                "storageFormat": "ndjson"
            }
        });
        RunManifest::from_json(&raw.to_string()).expect("manifest")
    }

    #[test]
    fn posts_ingest_batch_to_http_endpoint() {
        let listener = match TcpListener::bind("127.0.0.1:0") {
            Ok(listener) => listener,
            Err(error) => {
                eprintln!("skipping local ingest listener test: {error}");
                return;
            }
        };
        let addr = listener.local_addr().expect("addr");
        let (tx, rx) = mpsc::channel();
        thread::spawn(move || {
            let (mut stream, _) = listener.accept().expect("accept");
            let request = read_http_request(&mut stream);
            stream
                .write_all(b"HTTP/1.1 202 Accepted\r\nContent-Length: 2\r\n\r\n{}")
                .expect("write");
            tx.send(request).expect("send");
        });

        let run = generate_run(
            &manifest(),
            WorkerSelection::One(crate::manifest::WorkerKind::Scheduler),
            Some(1),
        )
        .expect("run");
        post_ingest(
            &run,
            &format!("http://{addr}/internal/simops/runs/RUN-TEST-001/ingest"),
            "secret-token",
        )
        .expect("post ingest");

        let request = rx.recv().expect("request");
        assert!(request.starts_with("POST /internal/simops/runs/RUN-TEST-001/ingest HTTP/1.1"));
        assert!(request.contains("X-Simops-Ingest-Token: secret-token"));
        assert!(request.contains("\"frames\""));
    }

    #[test]
    fn rejects_bad_ingest_status() {
        let listener = match TcpListener::bind("127.0.0.1:0") {
            Ok(listener) => listener,
            Err(error) => {
                eprintln!("skipping local ingest listener test: {error}");
                return;
            }
        };
        let addr = listener.local_addr().expect("addr");
        thread::spawn(move || {
            let (mut stream, _) = listener.accept().expect("accept");
            let _ = read_http_request(&mut stream);
            stream
                .write_all(b"HTTP/1.1 500 Internal Server Error\r\nContent-Length: 0\r\n\r\n")
                .expect("write");
        });

        let run = generate_run(
            &manifest(),
            WorkerSelection::One(crate::manifest::WorkerKind::Scheduler),
            Some(1),
        )
        .expect("run");
        let error = post_ingest(
            &run,
            &format!("http://{addr}/internal/simops/runs/RUN-TEST-001/ingest"),
            "secret-token",
        )
        .expect_err("bad status should fail");
        assert!(error.to_string().contains("non-success status"));
    }

    fn read_http_request(stream: &mut TcpStream) -> String {
        let mut bytes = Vec::new();
        let mut buffer = [0u8; 1024];
        let header_end = loop {
            let count = stream.read(&mut buffer).expect("read");
            assert!(count > 0, "connection closed before HTTP headers completed");
            bytes.extend_from_slice(&buffer[..count]);
            if let Some(index) = find_header_end(&bytes) {
                break index;
            }
        };

        let headers = String::from_utf8_lossy(&bytes[..header_end]).to_string();
        let content_length = content_length(&headers);
        let body_start = header_end + 4;
        while bytes.len().saturating_sub(body_start) < content_length {
            let count = stream.read(&mut buffer).expect("read body");
            assert!(count > 0, "connection closed before HTTP body completed");
            bytes.extend_from_slice(&buffer[..count]);
        }
        String::from_utf8_lossy(&bytes).to_string()
    }

    fn find_header_end(bytes: &[u8]) -> Option<usize> {
        bytes.windows(4).position(|window| window == b"\r\n\r\n")
    }

    fn content_length(headers: &str) -> usize {
        headers
            .lines()
            .find_map(|line| {
                let (name, value) = line.split_once(':')?;
                if name.eq_ignore_ascii_case("content-length") {
                    return value.trim().parse().ok();
                }
                None
            })
            .unwrap_or(0)
    }
}
