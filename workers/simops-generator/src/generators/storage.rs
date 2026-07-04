use serde::Serialize;

use crate::Result;
use crate::generators::{FrameContext, clamp, round1};

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct StoragePayload {
    parallel_file_system: ParallelFileSystem,
    burst_buffer: BurstBuffer,
    nvme_of: NvmeOf,
    checkpoint: Checkpoint,
    storage_targets: Vec<StorageTarget>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct ParallelFileSystem {
    read_iops: f64,
    write_iops: f64,
    metadata_ops: f64,
    health: &'static str,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct BurstBuffer {
    write_throughput_gbps: f64,
    flush_throughput_gbps: f64,
    queued_gi_b: f64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct NvmeOf {
    cache_saturation_pct: f64,
    write_amp: f64,
    target_queue_depth: u32,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct Checkpoint {
    checkpoint_id: &'static str,
    progress_pct: f64,
    eta_sec: f64,
    status: &'static str,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct StorageTarget {
    target_id: &'static str,
    role: &'static str,
    degradation_pct: f64,
    read_latency_ms: f64,
    write_latency_ms: f64,
    saturation_pct: f64,
}

pub fn generate(context: &FrameContext<'_>) -> Result<serde_json::Value> {
    let pressure = context.pressure;
    let progress = if context.manifest.runtime_limit_sec == 0 {
        0.0
    } else {
        context.offset_sec / f64::from(context.manifest.runtime_limit_sec)
    };
    let cache_saturation = round1(clamp(28.0 + pressure * 58.0, 0.0, 100.0));
    let health = if pressure >= 0.76 {
        "degraded"
    } else if pressure >= 0.52 {
        "watch"
    } else {
        "nominal"
    };

    let payload = StoragePayload {
        parallel_file_system: ParallelFileSystem {
            read_iops: round1(clamp(190_000.0 - pressure * 55_000.0, 0.0, 1_000_000.0)),
            write_iops: round1(clamp(134_000.0 - pressure * 78_000.0, 0.0, 1_000_000.0)),
            metadata_ops: round1(clamp(38_000.0 + pressure * 52_000.0, 0.0, 500_000.0)),
            health,
        },
        burst_buffer: BurstBuffer {
            write_throughput_gbps: round1(clamp(74.0 - pressure * 38.0, 0.0, 500.0)),
            flush_throughput_gbps: round1(clamp(66.0 - pressure * 42.0, 0.0, 500.0)),
            queued_gi_b: round1(clamp(640.0 + pressure * 3_200.0, 0.0, 100_000.0)),
        },
        nvme_of: NvmeOf {
            cache_saturation_pct: cache_saturation,
            write_amp: round1(clamp(1.4 + pressure * 2.4, 1.0, 20.0)),
            target_queue_depth: clamp(160.0 + pressure * 720.0, 0.0, 4096.0).round() as u32,
        },
        checkpoint: Checkpoint {
            checkpoint_id: "ckpt-404-a",
            progress_pct: round1(clamp(18.0 + progress * 78.0, 0.0, 100.0)),
            eta_sec: round1(clamp(
                f64::from(context.manifest.runtime_limit_sec).max(1.0) - context.offset_sec
                    + pressure * 96.0,
                0.0,
                3600.0,
            )),
            status: if pressure >= 0.82 {
                "stalled"
            } else if pressure >= 0.55 {
                "flushing"
            } else {
                "writing"
            },
        },
        storage_targets: vec![
            storage_target("ost-01", "object", pressure, 0.8),
            storage_target("ost-02", "object", pressure, 1.2),
            storage_target("mdt-01", "metadata", pressure, 0.6),
        ],
    };

    Ok(serde_json::to_value(payload)?)
}

fn storage_target(
    target_id: &'static str,
    role: &'static str,
    pressure: f64,
    multiplier: f64,
) -> StorageTarget {
    StorageTarget {
        target_id,
        role,
        degradation_pct: round1(clamp(6.0 + pressure * 42.0 * multiplier, 0.0, 100.0)),
        read_latency_ms: round1(clamp(1.8 + pressure * 7.0 * multiplier, 0.0, 1000.0)),
        write_latency_ms: round1(clamp(3.2 + pressure * 24.0 * multiplier, 0.0, 1000.0)),
        saturation_pct: round1(clamp(34.0 + pressure * 56.0 * multiplier, 0.0, 100.0)),
    }
}
