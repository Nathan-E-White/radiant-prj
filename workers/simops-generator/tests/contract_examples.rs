use std::collections::HashMap;
use std::fs;
use std::process::Command;

use serde_json::{Value, json};

#[test]
fn emits_scheduler_drift_contract_frames_and_summary() {
    let binary = env!("CARGO_BIN_EXE_simops-generator");
    let manifest = "../../examples/simulation-ops/run-manifest.scheduler-drift.json";
    let summary_path = std::env::temp_dir().join(format!(
        "simops-generator-summary-{}.json",
        std::process::id()
    ));
    let results_path = std::env::temp_dir().join(format!(
        "simops-generator-results-{}.ndjson",
        std::process::id()
    ));

    let output = Command::new(binary)
        .args([
            "--manifest",
            manifest,
            "--worker",
            "all",
            "--frames",
            "2",
            "--output",
            "-",
            "--summary",
            summary_path.to_str().expect("utf8 temp path"),
            "--results-output",
            results_path.to_str().expect("utf8 results path"),
        ])
        .output()
        .expect("run simops-generator");

    assert!(
        output.status.success(),
        "generator failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let stdout = String::from_utf8(output.stdout).expect("utf8 stdout");
    let frames: Vec<Value> = stdout
        .lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| serde_json::from_str(line).expect("valid frame json"))
        .collect();

    assert_eq!(frames.len(), 8);
    let mut sequences_by_worker: HashMap<String, Vec<u64>> = HashMap::new();

    for frame in &frames {
        assert_eq!(frame["schemaVersion"], "simops.telemetry.v1");
        let worker_id = frame["workerId"].as_str().expect("workerId").to_string();
        sequences_by_worker
            .entry(worker_id)
            .or_default()
            .push(frame["sequence"].as_u64().expect("sequence"));
    }

    assert_eq!(sequences_by_worker.len(), 4);
    for sequences in sequences_by_worker.values_mut() {
        sequences.sort_unstable();
        assert_eq!(sequences, &vec![1, 2]);
    }

    let summary_text = fs::read_to_string(&summary_path).expect("summary file");
    let summary: Value = serde_json::from_str(&summary_text).expect("valid summary json");
    assert_eq!(summary["schemaVersion"], "simops.run-summary.v1");
    assert_eq!(summary["frameCount"], 8);
    assert_eq!(summary["workers"].as_array().expect("workers").len(), 4);
    assert!(
        !summary["aggregateMetrics"]
            .as_array()
            .expect("aggregate metrics")
            .is_empty()
    );

    let results_text = fs::read_to_string(&results_path).expect("results file");
    let results: Vec<Value> = results_text
        .lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| serde_json::from_str(line).expect("valid result json"))
        .collect();
    assert_eq!(results.len(), 2);
    for result in &results {
        assert_eq!(result["schemaVersion"], "simops.result.v1");
        assert_eq!(result["valueBasis"], "simulated");
        assert_eq!(result["values"][0]["valueId"], "VAL-SIMULATED-FORECAST-MARGIN");
    }

    let _ = fs::remove_file(summary_path);
    let _ = fs::remove_file(results_path);
}

#[test]
fn stochastic_manifest_records_dropped_logical_frames() {
    let binary = env!("CARGO_BIN_EXE_simops-generator");
    let manifest_path = std::env::temp_dir().join(format!(
        "simops-generator-stochastic-manifest-{}.json",
        std::process::id()
    ));
    let summary_path = std::env::temp_dir().join(format!(
        "simops-generator-stochastic-summary-{}.json",
        std::process::id()
    ));
    let manifest = json!({
        "schemaVersion": "simops.run-manifest.v1",
        "runId": "RUN-HPC-DROP-ALL-001",
        "scenarioId": "scheduler-drift",
        "createdAt": "2026-07-04T18:00:00.000Z",
        "lifecycle": "created",
        "workbenchAnchor": {
            "jobId": "JOB-HPC-DROP-ALL",
            "evidencePackId": "EP-HPC-DROP-ALL",
            "gatewayEvidenceId": "SLURM-GATEWAY-DROP-ALL"
        },
        "runtimeLimitSec": 20,
        "transportBinding": "ndjson",
        "randomization": {
            "mode": "correlated-pressure",
            "randomSeed": 40401,
            "pressureCurve": [
                { "offsetSec": 0, "pressure": 0.2 },
                { "offsetSec": 20, "pressure": 0.8 }
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
                    "probability": { "base": 0.0, "pressureSlope": 0.0, "min": 0.0, "max": 0.0 }
                },
                "delay": {
                    "distribution": "gamma",
                    "sourceLagMs": { "base": 10.0, "pressureSlope": 20.0, "min": 1.0, "max": 100.0 },
                    "collectorLagMs": { "base": 20.0, "pressureSlope": 40.0, "min": 1.0, "max": 200.0 },
                    "shape": 2.0
                },
                "workerSensitivity": {
                    "scheduler": 1.0,
                    "storage": 1.0,
                    "burst": 1.0,
                    "fabric": 1.0
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
                "artifactId": "simops-run-manifest.drop-all",
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
    fs::write(
        &manifest_path,
        serde_json::to_string_pretty(&manifest).expect("manifest json"),
    )
    .expect("write manifest");

    let output = Command::new(binary)
        .args([
            "--manifest",
            manifest_path.to_str().expect("utf8 manifest path"),
            "--worker",
            "all",
            "--frames",
            "2",
            "--output",
            "-",
            "--summary",
            summary_path.to_str().expect("utf8 summary path"),
        ])
        .output()
        .expect("run simops-generator");

    assert!(
        output.status.success(),
        "generator failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert!(
        String::from_utf8(output.stdout)
            .expect("utf8 stdout")
            .trim()
            .is_empty()
    );

    let summary_text = fs::read_to_string(&summary_path).expect("summary file");
    let summary: Value = serde_json::from_str(&summary_text).expect("valid summary json");
    assert_eq!(summary["frameCount"], 0);
    assert_eq!(summary["requestedFrameCount"], 2);
    assert_eq!(summary["droppedFrameCount"], 2);
    assert_eq!(summary["workers"][0]["frames"], 0);
    assert_eq!(summary["workers"][0]["droppedFrameCount"], 2);

    let _ = fs::remove_file(manifest_path);
    let _ = fs::remove_file(summary_path);
}
