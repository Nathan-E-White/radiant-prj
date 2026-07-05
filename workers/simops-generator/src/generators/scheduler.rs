use serde::Serialize;

use crate::Result;
use crate::generators::{FrameContext, clamp, round1};

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct SchedulerPayload {
    slurm: SlurmState,
    mpi_apps: Vec<MpiApp>,
    barrier_wait_ms: PercentileSamples,
    sync_latency_us: Percentiles,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct SlurmState {
    job_id: String,
    partition: String,
    allocation_state: &'static str,
    queue_lanes: Vec<QueueLane>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct QueueLane {
    queue: &'static str,
    state: &'static str,
    offset_start_sec: f64,
    duration_sec: f64,
    allocated_nodes: u32,
    pending_nodes: u32,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct MpiApp {
    app: &'static str,
    ranks: u32,
    rank_start: u32,
    rank_end: u32,
    state: &'static str,
}

#[derive(Serialize)]
struct Percentiles {
    p50: f64,
    p95: f64,
    max: f64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct PercentileSamples {
    p50: f64,
    p95: f64,
    max: f64,
    samples: Vec<WaitSample>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct WaitSample {
    offset_sec: f64,
    wait_ms: f64,
}

pub fn generate(context: &FrameContext<'_>) -> Result<serde_json::Value> {
    let pressure = context.pressure;
    let barrier_p95 = round1(clamp(24.0 + pressure * 210.0, 0.0, 1000.0));
    let barrier_p50 = round1(clamp(barrier_p95 * 0.42, 0.0, 500.0));
    let barrier_max = round1(clamp(barrier_p95 * 1.28, 0.0, 1500.0));
    let sync_p95 = round1(clamp(260.0 + pressure * 1900.0, 0.0, 5000.0));
    let sync_p50 = round1(clamp(sync_p95 * 0.32, 0.0, 2000.0));
    let sync_max = round1(clamp(sync_p95 * 1.24, 0.0, 10000.0));
    let pending_nodes = if pressure >= 0.7 {
        4
    } else if pressure >= 0.45 {
        2
    } else {
        0
    };
    let allocation_state = if pressure >= 0.72 {
        "held"
    } else if pressure >= 0.58 {
        "degraded"
    } else {
        "running"
    };
    let lane_state = if pressure >= 0.72 { "held" } else { "running" };

    let payload = SchedulerPayload {
        slurm: SlurmState {
            job_id: context.manifest.workbench_anchor.job_id.clone(),
            partition: "cpu-long".to_string(),
            allocation_state,
            queue_lanes: vec![
                QueueLane {
                    queue: "cpu-long",
                    state: lane_state,
                    offset_start_sec: round1(context.offset_sec),
                    duration_sec: round1(clamp(32.0 + pressure * 46.0, 0.0, 600.0)),
                    allocated_nodes: 12_u32.saturating_sub(pending_nodes),
                    pending_nodes,
                },
                QueueLane {
                    queue: "gpu",
                    state: "queued",
                    offset_start_sec: round1(context.offset_sec + 4.0),
                    duration_sec: round1(clamp(30.0 + pressure * 30.0, 0.0, 600.0)),
                    allocated_nodes: 0,
                    pending_nodes: 4,
                },
            ],
        },
        mpi_apps: vec![
            MpiApp {
                app: "neutronics",
                ranks: 512,
                rank_start: 0,
                rank_end: 511,
                state: if pressure >= 0.68 {
                    "waiting"
                } else {
                    "running"
                },
            },
            MpiApp {
                app: "fluids",
                ranks: 256,
                rank_start: 512,
                rank_end: 767,
                state: if pressure >= 0.55 {
                    "waiting"
                } else {
                    "running"
                },
            },
            MpiApp {
                app: "thermal",
                ranks: 128,
                rank_start: 768,
                rank_end: 895,
                state: "running",
            },
        ],
        barrier_wait_ms: PercentileSamples {
            p50: barrier_p50,
            p95: barrier_p95,
            max: barrier_max,
            samples: vec![
                WaitSample {
                    offset_sec: round1(context.offset_sec),
                    wait_ms: round1(clamp(barrier_p50 * 0.8, 0.0, 1500.0)),
                },
                WaitSample {
                    offset_sec: round1(context.offset_sec + 1.0),
                    wait_ms: barrier_p50,
                },
                WaitSample {
                    offset_sec: round1(context.offset_sec + 2.0),
                    wait_ms: barrier_p95,
                },
            ],
        },
        sync_latency_us: Percentiles {
            p50: sync_p50,
            p95: sync_p95,
            max: sync_max,
        },
    };

    Ok(serde_json::to_value(payload)?)
}
