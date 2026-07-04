use serde::Serialize;

use crate::Result;
use crate::generators::{FrameContext, clamp, round1, round3};

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct BurstPayload {
    mesh: Mesh,
    parallel_cluster: ParallelCluster,
    efa: Efa,
    spot_cost: SpotCost,
    topology: Topology,
}

#[derive(Serialize)]
struct Mesh {
    hotspot: Hotspot,
    cells: Vec<MeshCell>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct Hotspot {
    active: bool,
    cell_id: &'static str,
    intensity: f64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct MeshCell {
    cell_id: &'static str,
    x: u32,
    y: u32,
    load_pct: f64,
    thermal_proxy: f64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct ParallelCluster {
    state: &'static str,
    local_nodes: u32,
    cloud_desired: u32,
    cloud_pending: u32,
    cloud_ready: u32,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct Efa {
    packet_drop_rate: f64,
    retransmit_pct: f64,
    latency_us_p95: f64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct SpotCost {
    usd_per_hour: f64,
    estimated_run_usd: f64,
    interruption_risk_pct: f64,
}

#[derive(Serialize)]
struct Topology {
    nodes: Vec<TopologyNode>,
    links: Vec<TopologyLink>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct TopologyNode {
    node_id: &'static str,
    node_type: &'static str,
    state: &'static str,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct TopologyLink {
    source: &'static str,
    target: &'static str,
    latency_ms: f64,
    utilization_pct: f64,
}

pub fn generate(context: &FrameContext<'_>) -> Result<serde_json::Value> {
    let pressure = context.pressure;
    let cloud_desired = if pressure >= 0.58 {
        clamp((pressure * 10.0).ceil(), 0.0, 1024.0) as u32
    } else {
        0
    };
    let cloud_ready = cloud_desired.saturating_sub(if pressure >= 0.76 { 2 } else { 0 });
    let cluster_state = if pressure >= 0.8 {
        "active"
    } else if pressure >= 0.65 {
        "scaling"
    } else if pressure >= 0.5 {
        "requesting"
    } else {
        "inactive"
    };

    let payload = BurstPayload {
        mesh: Mesh {
            hotspot: Hotspot {
                active: pressure >= 0.58,
                cell_id: "cell-07-08",
                intensity: round3(clamp(pressure, 0.0, 1.0)),
            },
            cells: vec![
                mesh_cell("cell-06-08", 6, 8, pressure, 0.82),
                mesh_cell("cell-07-08", 7, 8, pressure, 1.0),
                mesh_cell("cell-08-08", 8, 8, pressure, 0.88),
            ],
        },
        parallel_cluster: ParallelCluster {
            state: cluster_state,
            local_nodes: 16,
            cloud_desired,
            cloud_pending: cloud_desired.saturating_sub(cloud_ready),
            cloud_ready,
        },
        efa: Efa {
            packet_drop_rate: round3(clamp(0.001 + pressure * 0.018, 0.0, 1.0)),
            retransmit_pct: round1(clamp(0.25 + pressure * 8.0, 0.0, 100.0)),
            latency_us_p95: round1(clamp(380.0 + pressure * 1400.0, 0.0, 10000.0)),
        },
        spot_cost: SpotCost {
            usd_per_hour: round1(clamp(f64::from(cloud_ready) * 7.5, 0.0, 10000.0)),
            estimated_run_usd: round1(clamp(f64::from(cloud_ready) * 1.4, 0.0, 10000.0)),
            interruption_risk_pct: round1(clamp(pressure * 18.0, 0.0, 100.0)),
        },
        topology: topology(cloud_ready, pressure),
    };

    Ok(serde_json::to_value(payload)?)
}

fn mesh_cell(cell_id: &'static str, x: u32, y: u32, pressure: f64, multiplier: f64) -> MeshCell {
    MeshCell {
        cell_id,
        x,
        y,
        load_pct: round1(clamp(38.0 + pressure * 58.0 * multiplier, 0.0, 100.0)),
        thermal_proxy: round3(clamp(0.25 + pressure * 0.62 * multiplier, 0.0, 1.0)),
    }
}

fn topology(cloud_ready: u32, pressure: f64) -> Topology {
    let mut nodes = vec![
        TopologyNode {
            node_id: "local-a",
            node_type: "local",
            state: "active",
        },
        TopologyNode {
            node_id: "local-b",
            node_type: "local",
            state: "active",
        },
        TopologyNode {
            node_id: "gateway",
            node_type: "gateway",
            state: "ready",
        },
    ];

    if cloud_ready > 0 {
        nodes.push(TopologyNode {
            node_id: "cloud-a",
            node_type: "cloud",
            state: "active",
        });
    }

    let mut links = vec![
        TopologyLink {
            source: "local-a",
            target: "gateway",
            latency_ms: round1(clamp(0.4 + pressure * 1.4, 0.0, 1000.0)),
            utilization_pct: round1(clamp(36.0 + pressure * 44.0, 0.0, 100.0)),
        },
        TopologyLink {
            source: "local-b",
            target: "gateway",
            latency_ms: round1(clamp(0.5 + pressure * 1.6, 0.0, 1000.0)),
            utilization_pct: round1(clamp(40.0 + pressure * 42.0, 0.0, 100.0)),
        },
    ];

    if cloud_ready > 0 {
        links.push(TopologyLink {
            source: "gateway",
            target: "cloud-a",
            latency_ms: round1(clamp(14.0 + pressure * 30.0, 0.0, 1000.0)),
            utilization_pct: round1(clamp(44.0 + pressure * 38.0, 0.0, 100.0)),
        });
    }

    Topology { nodes, links }
}
