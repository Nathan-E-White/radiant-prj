use serde::Serialize;

use crate::Result;
use crate::generators::{FrameContext, clamp, round1, round3};

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct FabricPayload {
    nodes: Vec<FabricNode>,
    links: Vec<FabricLink>,
    ib_counters: IbCounters,
    message_sizes: Vec<MessageSize>,
    nonblocking_comm: NonblockingComm,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct FabricNode {
    node_id: &'static str,
    rack: &'static str,
    role: &'static str,
    load_pct: f64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct FabricLink {
    source: &'static str,
    target: &'static str,
    utilization_pct: f64,
    latency_us: f64,
    error_rate: f64,
    temperature_class: &'static str,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct IbCounters {
    port_xmit_wait: u64,
    symbol_errors: u64,
    link_downed: u64,
    vl15_dropped: u64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct MessageSize {
    bucket_bytes: u64,
    count: u64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct NonblockingComm {
    posted_receives: u64,
    pending_sends: u64,
    completion_overhead_pct: f64,
    progress_stall_ms_p95: f64,
}

pub fn generate(context: &FrameContext<'_>) -> Result<serde_json::Value> {
    let pressure = context.pressure;
    let temperature_class = if pressure >= 0.76 {
        "hot"
    } else if pressure >= 0.5 {
        "warm"
    } else {
        "cool"
    };

    let payload = FabricPayload {
        nodes: vec![
            fabric_node("rank-001", "rack-a", "rank-host", pressure, 1.0),
            fabric_node("rank-017", "rack-a", "rank-host", pressure, 0.75),
            fabric_node("io-001", "rack-b", "io", pressure, 0.62),
        ],
        links: vec![
            FabricLink {
                source: "rank-001",
                target: "rank-017",
                utilization_pct: round1(clamp(44.0 + pressure * 42.0, 0.0, 100.0)),
                latency_us: round1(clamp(88.0 + pressure * 620.0, 0.0, 10000.0)),
                error_rate: round3(clamp(0.0002 + pressure * 0.006, 0.0, 1.0)),
                temperature_class,
            },
            FabricLink {
                source: "rank-017",
                target: "io-001",
                utilization_pct: round1(clamp(52.0 + pressure * 36.0, 0.0, 100.0)),
                latency_us: round1(clamp(120.0 + pressure * 720.0, 0.0, 10000.0)),
                error_rate: round3(clamp(0.0004 + pressure * 0.007, 0.0, 1.0)),
                temperature_class,
            },
        ],
        ib_counters: IbCounters {
            port_xmit_wait: clamp(12_000.0 + pressure * 120_000.0, 0.0, 100_000_000.0).round()
                as u64,
            symbol_errors: clamp(3.0 + pressure * 45.0, 0.0, 1_000_000.0).round() as u64,
            link_downed: if pressure >= 0.85 { 1 } else { 0 },
            vl15_dropped: clamp(6.0 + pressure * 120.0, 0.0, 1_000_000.0).round() as u64,
        },
        message_sizes: vec![
            MessageSize {
                bucket_bytes: 4096,
                count: clamp(8_400.0 + pressure * 3_000.0, 0.0, 10_000_000.0).round() as u64,
            },
            MessageSize {
                bucket_bytes: 65_536,
                count: clamp(3_200.0 + pressure * 2_100.0, 0.0, 10_000_000.0).round() as u64,
            },
            MessageSize {
                bucket_bytes: 1_048_576,
                count: clamp(420.0 + pressure * 680.0, 0.0, 10_000_000.0).round() as u64,
            },
        ],
        nonblocking_comm: NonblockingComm {
            posted_receives: clamp(178_000.0 + pressure * 90_000.0, 0.0, 10_000_000.0).round()
                as u64,
            pending_sends: clamp(720.0 + pressure * 3_400.0, 0.0, 10_000_000.0).round() as u64,
            completion_overhead_pct: round1(clamp(6.0 + pressure * 28.0, 0.0, 100.0)),
            progress_stall_ms_p95: round1(clamp(9.0 + pressure * 92.0, 0.0, 10000.0)),
        },
    };

    Ok(serde_json::to_value(payload)?)
}

fn fabric_node(
    node_id: &'static str,
    rack: &'static str,
    role: &'static str,
    pressure: f64,
    multiplier: f64,
) -> FabricNode {
    FabricNode {
        node_id,
        rack,
        role,
        load_pct: round1(clamp(34.0 + pressure * 50.0 * multiplier, 0.0, 100.0)),
    }
}
