use serde::{Deserialize, Serialize};

use crate::{Result, SimopsError};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RunManifest {
    pub schema_version: String,
    pub run_id: String,
    pub scenario_id: ScenarioId,
    pub created_at: String,
    pub lifecycle: ManifestLifecycle,
    pub workbench_anchor: WorkbenchAnchor,
    pub runtime_limit_sec: u32,
    pub transport_binding: TransportBinding,
    pub randomization: Randomization,
    pub workers: Vec<WorkerDeclaration>,
    pub artifacts: Vec<ManifestArtifact>,
    pub provenance: ManifestProvenance,
}

impl RunManifest {
    pub fn from_json(input: &str) -> Result<Self> {
        let manifest: Self = serde_json::from_str(input)?;
        manifest.validate()?;
        Ok(manifest)
    }

    pub fn validate(&self) -> Result<()> {
        if self.schema_version != "simops.run-manifest.v1" {
            return Err(SimopsError::new(format!(
                "unsupported manifest schemaVersion {}",
                self.schema_version
            )));
        }

        if self.randomization.mode != "correlated-pressure" {
            return Err(SimopsError::new(format!(
                "unsupported randomization mode {}",
                self.randomization.mode
            )));
        }

        if self.workers.is_empty() {
            return Err(SimopsError::new(
                "manifest must declare at least one worker",
            ));
        }

        for worker in &self.workers {
            let expected = worker.worker_kind.expected_payload_type();
            if worker.payload_type != expected {
                return Err(SimopsError::new(format!(
                    "worker {} has payload {:?}, expected {:?}",
                    worker.worker_id, worker.payload_type, expected
                )));
            }
        }

        if self.randomization.pressure_curve.len() < 2 {
            return Err(SimopsError::new(
                "manifest pressureCurve must contain at least two points",
            ));
        }

        let mut previous_offset = None;
        for point in &self.randomization.pressure_curve {
            if let Some(previous) = previous_offset {
                if point.offset_sec <= previous {
                    return Err(SimopsError::new(
                        "manifest pressureCurve offsetSec values must increase",
                    ));
                }
            }
            previous_offset = Some(point.offset_sec);
        }

        if let Some(profile) = &self.randomization.distribution_profile {
            profile.validate()?;
        }

        Ok(())
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkbenchAnchor {
    pub job_id: String,
    pub evidence_pack_id: String,
    pub gateway_evidence_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Randomization {
    pub mode: String,
    #[serde(default)]
    pub random_seed: Option<u64>,
    pub pressure_curve: Vec<PressurePoint>,
    pub baseline: Vec<BaselineMetric>,
    pub couplings: Vec<Coupling>,
    pub events: Vec<ScenarioEvent>,
    pub bounds: Vec<MetricBound>,
    #[serde(default)]
    pub distribution_profile: Option<DistributionProfile>,
}

impl Randomization {
    pub fn pressure_at(&self, offset_sec: f64) -> f64 {
        if self.pressure_curve.is_empty() {
            return 0.0;
        }

        let offset_sec = offset_sec.max(0.0);
        let first = &self.pressure_curve[0];
        if offset_sec <= f64::from(first.offset_sec) {
            return first.pressure.clamp(0.0, 1.0);
        }

        for window in self.pressure_curve.windows(2) {
            let left = &window[0];
            let right = &window[1];
            let left_offset = f64::from(left.offset_sec);
            let right_offset = f64::from(right.offset_sec);

            if offset_sec <= right_offset {
                let span = (right_offset - left_offset).max(1.0);
                let t = ((offset_sec - left_offset) / span).clamp(0.0, 1.0);
                return (left.pressure + (right.pressure - left.pressure) * t).clamp(0.0, 1.0);
            }
        }

        self.pressure_curve
            .last()
            .map(|point| point.pressure.clamp(0.0, 1.0))
            .unwrap_or(0.0)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DistributionProfile {
    #[serde(default)]
    pub emission: Option<EmissionProfile>,
    #[serde(default)]
    pub delay: Option<DelayProfile>,
    #[serde(default)]
    pub burst_loss: Option<BurstLossProfile>,
    #[serde(default)]
    pub worker_sensitivity: WorkerSensitivity,
}

impl DistributionProfile {
    pub fn validate(&self) -> Result<()> {
        if let Some(emission) = &self.emission {
            emission
                .probability
                .validate_probability("randomization.distributionProfile.emission.probability")?;
        }

        if let Some(delay) = &self.delay {
            delay
                .source_lag_ms
                .validate_non_negative("randomization.distributionProfile.delay.sourceLagMs")?;
            delay
                .collector_lag_ms
                .validate_non_negative("randomization.distributionProfile.delay.collectorLagMs")?;

            if !delay.sigma.is_finite() || delay.sigma <= 0.0 {
                return Err(SimopsError::new(
                    "randomization.distributionProfile.delay.sigma must be greater than zero",
                ));
            }

            if !delay.shape.is_finite() || delay.shape <= 0.0 {
                return Err(SimopsError::new(
                    "randomization.distributionProfile.delay.shape must be greater than zero",
                ));
            }
        }

        if let Some(burst_loss) = &self.burst_loss {
            burst_loss
                .good_to_bad
                .validate_probability("randomization.distributionProfile.burstLoss.goodToBad")?;
            burst_loss
                .bad_to_good
                .validate_probability("randomization.distributionProfile.burstLoss.badToGood")?;
            validate_probability(
                burst_loss.bad_emit_probability,
                "randomization.distributionProfile.burstLoss.badEmitProbability",
            )?;
        }

        for (label, value) in [
            ("scheduler", self.worker_sensitivity.scheduler),
            ("storage", self.worker_sensitivity.storage),
            ("burst", self.worker_sensitivity.burst),
            ("fabric", self.worker_sensitivity.fabric),
        ] {
            if !value.is_finite() || value < 0.0 {
                return Err(SimopsError::new(format!(
                    "randomization.distributionProfile.workerSensitivity.{label} must be finite and non-negative"
                )));
            }
        }

        Ok(())
    }

    pub fn worker_multiplier(&self, worker_kind: WorkerKind) -> f64 {
        match worker_kind {
            WorkerKind::Scheduler => self.worker_sensitivity.scheduler,
            WorkerKind::Storage => self.worker_sensitivity.storage,
            WorkerKind::Burst => self.worker_sensitivity.burst,
            WorkerKind::Fabric => self.worker_sensitivity.fabric,
        }
        .clamp(0.0, 10.0)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EmissionProfile {
    pub probability: PressureScaledParameter,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DelayProfile {
    pub distribution: DelayDistributionKind,
    pub source_lag_ms: PressureScaledParameter,
    pub collector_lag_ms: PressureScaledParameter,
    #[serde(default = "default_delay_sigma")]
    pub sigma: f64,
    #[serde(default = "default_delay_shape")]
    pub shape: f64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DelayDistributionKind {
    Lognormal,
    Gamma,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BurstLossProfile {
    pub good_to_bad: PressureScaledParameter,
    pub bad_to_good: PressureScaledParameter,
    pub bad_emit_probability: f64,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkerSensitivity {
    #[serde(default = "default_worker_sensitivity")]
    pub scheduler: f64,
    #[serde(default = "default_worker_sensitivity")]
    pub storage: f64,
    #[serde(default = "default_worker_sensitivity")]
    pub burst: f64,
    #[serde(default = "default_worker_sensitivity")]
    pub fabric: f64,
}

impl Default for WorkerSensitivity {
    fn default() -> Self {
        Self {
            scheduler: default_worker_sensitivity(),
            storage: default_worker_sensitivity(),
            burst: default_worker_sensitivity(),
            fabric: default_worker_sensitivity(),
        }
    }
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PressureScaledParameter {
    pub base: f64,
    #[serde(default)]
    pub pressure_slope: f64,
    pub min: f64,
    pub max: f64,
}

impl PressureScaledParameter {
    pub fn evaluate(self, pressure: f64) -> f64 {
        (self.base + self.pressure_slope * pressure.clamp(0.0, 1.0)).clamp(self.min, self.max)
    }

    fn validate(self, label: &str) -> Result<()> {
        if !self.base.is_finite()
            || !self.pressure_slope.is_finite()
            || !self.min.is_finite()
            || !self.max.is_finite()
        {
            return Err(SimopsError::new(format!("{label} values must be finite")));
        }

        if self.min > self.max {
            return Err(SimopsError::new(format!(
                "{label} min must be less than or equal to max"
            )));
        }

        Ok(())
    }

    fn validate_non_negative(self, label: &str) -> Result<()> {
        self.validate(label)?;
        if self.min < 0.0 {
            return Err(SimopsError::new(format!(
                "{label} min must be non-negative"
            )));
        }
        Ok(())
    }

    fn validate_probability(self, label: &str) -> Result<()> {
        self.validate(label)?;
        if self.min < 0.0 || self.max > 1.0 {
            return Err(SimopsError::new(format!(
                "{label} min and max must stay within 0..1"
            )));
        }
        Ok(())
    }
}

fn validate_probability(value: f64, label: &str) -> Result<()> {
    if !value.is_finite() || !(0.0..=1.0).contains(&value) {
        return Err(SimopsError::new(format!("{label} must stay within 0..1")));
    }
    Ok(())
}

fn default_delay_sigma() -> f64 {
    0.35
}

fn default_delay_shape() -> f64 {
    2.0
}

fn default_worker_sensitivity() -> f64 {
    1.0
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PressurePoint {
    pub offset_sec: u32,
    pub pressure: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BaselineMetric {
    pub metric_path: String,
    pub nominal_min: f64,
    pub nominal_max: f64,
    pub unit: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Coupling {
    pub from: String,
    pub to: String,
    pub effect: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ScenarioEvent {
    pub offset_sec: u32,
    pub event_type: String,
    pub description: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MetricBound {
    pub metric_path: String,
    pub min: f64,
    pub max: f64,
    pub unit: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkerDeclaration {
    pub worker_id: String,
    pub worker_kind: WorkerKind,
    pub payload_type: PayloadType,
    pub emit_hz: f64,
    pub panel_target: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ManifestArtifact {
    pub artifact_id: String,
    pub path: String,
    pub media_type: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ManifestProvenance {
    pub generated_by: String,
    pub data_class: String,
    pub storage_format: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum ScenarioId {
    Nominal,
    SchedulerDrift,
    CheckpointPressure,
    CloudBurst,
    FabricWarning,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum WorkerKind {
    Scheduler,
    Storage,
    Burst,
    Fabric,
}

impl WorkerKind {
    pub fn expected_payload_type(self) -> PayloadType {
        match self {
            WorkerKind::Scheduler => PayloadType::SchedulerCoScheduling,
            WorkerKind::Storage => PayloadType::CheckpointStorage,
            WorkerKind::Burst => PayloadType::ElasticBursting,
            WorkerKind::Fabric => PayloadType::FabricProfiler,
        }
    }

    pub fn as_str(self) -> &'static str {
        match self {
            WorkerKind::Scheduler => "scheduler",
            WorkerKind::Storage => "storage",
            WorkerKind::Burst => "burst",
            WorkerKind::Fabric => "fabric",
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum PayloadType {
    #[serde(rename = "schedulerCoScheduling")]
    SchedulerCoScheduling,
    #[serde(rename = "checkpointStorage")]
    CheckpointStorage,
    #[serde(rename = "elasticBursting")]
    ElasticBursting,
    #[serde(rename = "fabricProfiler")]
    FabricProfiler,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum ManifestLifecycle {
    Created,
    Starting,
    Streaming,
    Degraded,
    Complete,
    Failed,
    Stopped,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum TransportBinding {
    Ndjson,
    MoqWebtransport,
    Websocket,
    HttpBatch,
    LocalTcp,
    QuicWebtransport,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn interpolates_pressure_between_points() {
        let randomization = Randomization {
            mode: "correlated-pressure".to_string(),
            random_seed: None,
            pressure_curve: vec![
                PressurePoint {
                    offset_sec: 0,
                    pressure: 0.1,
                },
                PressurePoint {
                    offset_sec: 10,
                    pressure: 0.9,
                },
            ],
            baseline: vec![],
            couplings: vec![],
            events: vec![],
            bounds: vec![],
            distribution_profile: None,
        };

        assert!((randomization.pressure_at(5.0) - 0.5).abs() < 0.0001);
        assert!((randomization.pressure_at(20.0) - 0.9).abs() < 0.0001);
    }

    #[test]
    fn pressure_scaled_parameter_clamps_to_bounds() {
        let parameter = PressureScaledParameter {
            base: 0.9,
            pressure_slope: 0.4,
            min: 0.1,
            max: 1.0,
        };

        assert_eq!(parameter.evaluate(0.0), 0.9);
        assert_eq!(parameter.evaluate(1.0), 1.0);
    }

    #[test]
    fn maps_worker_kind_to_payload_type() {
        assert_eq!(
            WorkerKind::Scheduler.expected_payload_type(),
            PayloadType::SchedulerCoScheduling
        );
        assert_eq!(
            WorkerKind::Storage.expected_payload_type(),
            PayloadType::CheckpointStorage
        );
        assert_eq!(
            WorkerKind::Burst.expected_payload_type(),
            PayloadType::ElasticBursting
        );
        assert_eq!(
            WorkerKind::Fabric.expected_payload_type(),
            PayloadType::FabricProfiler
        );
    }
}
