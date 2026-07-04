use std::collections::HashMap;

use rand::rngs::StdRng;
use rand::{RngExt, SeedableRng};
use rand_distr::{Bernoulli, Distribution, Gamma, LogNormal};

use crate::generators::clamp;
use crate::manifest::{DelayDistributionKind, DelayProfile, RunManifest, WorkerDeclaration};
use crate::{Result, SimopsError};

const FNV_OFFSET: u64 = 0xcbf2_9ce4_8422_2325;
const FNV_PRIME: u64 = 0x0000_0100_0000_01b3;
const LATENT_JITTER_WIDTH: f64 = 0.07;
const BURST_PRESSURE_BONUS: f64 = 0.08;

#[derive(Debug, Clone, Copy, PartialEq)]
pub struct SampleOutcome {
    pub emitted: bool,
    pub pressure: f64,
    pub source_lag_ms: f64,
    pub collector_lag_ms: f64,
    pub received_lag_ms: Option<f64>,
    pub burst_bad: bool,
}

#[derive(Debug, Clone, Copy)]
struct LatentBucket {
    pressure_jitter: f64,
    burst_bad: bool,
}

pub struct ScenarioSampler<'a> {
    manifest: &'a RunManifest,
    seed: u64,
    latent_buckets: HashMap<u32, LatentBucket>,
    burst_states: Vec<bool>,
}

impl<'a> ScenarioSampler<'a> {
    pub fn new(manifest: &'a RunManifest) -> Self {
        Self {
            manifest,
            seed: manifest_seed(manifest),
            latent_buckets: HashMap::new(),
            burst_states: Vec::new(),
        }
    }

    pub fn sample(
        &mut self,
        worker: &WorkerDeclaration,
        sequence: u64,
        offset_sec: f64,
        base_pressure: f64,
    ) -> Result<SampleOutcome> {
        let Some(profile) = &self.manifest.randomization.distribution_profile else {
            return Ok(deterministic_sample(base_pressure, sequence));
        };

        let bucket = bucket_for_offset(offset_sec);
        let latent = self.latent_bucket(bucket)?;
        let mut pressure = clamp(base_pressure + latent.pressure_jitter, 0.0, 1.0);
        if latent.burst_bad {
            pressure = clamp(pressure + BURST_PRESSURE_BONUS, 0.0, 1.0);
        }
        pressure = clamp(
            pressure * profile.worker_multiplier(worker.worker_kind),
            0.0,
            1.0,
        );

        let mut emit_probability = profile
            .emission
            .as_ref()
            .map(|emission| emission.probability.evaluate(pressure))
            .unwrap_or(1.0);

        if latent.burst_bad {
            if let Some(burst_loss) = &profile.burst_loss {
                emit_probability = emit_probability.min(burst_loss.bad_emit_probability);
            }
        }

        let emitted = sample_bernoulli(
            emit_probability,
            self.rng_for("emission", bucket, worker.worker_id.as_str(), sequence),
        )?;

        let (source_lag_ms, collector_lag_ms, received_lag_ms) =
            if emitted && profile.delay.is_some() {
                let delay = profile.delay.as_ref().expect("checked delay");
                let mut source_rng =
                    self.rng_for("source-lag", bucket, worker.worker_id.as_str(), sequence);
                let mut collector_rng =
                    self.rng_for("collector-lag", bucket, worker.worker_id.as_str(), sequence);
                let source = sample_delay(
                    delay,
                    delay.source_lag_ms.evaluate(pressure),
                    &mut source_rng,
                )?;
                let collector = sample_delay(
                    delay,
                    delay.collector_lag_ms.evaluate(pressure),
                    &mut collector_rng,
                )?
                .max(source);
                (source, collector, Some(collector))
            } else {
                let deterministic = deterministic_sample(pressure, sequence);
                (
                    deterministic.source_lag_ms,
                    deterministic.collector_lag_ms,
                    None,
                )
            };

        Ok(SampleOutcome {
            emitted,
            pressure,
            source_lag_ms,
            collector_lag_ms,
            received_lag_ms,
            burst_bad: latent.burst_bad,
        })
    }

    fn latent_bucket(&mut self, bucket: u32) -> Result<LatentBucket> {
        if let Some(bucket) = self.latent_buckets.get(&bucket) {
            return Ok(*bucket);
        }

        self.ensure_burst_states(bucket)?;
        let mut rng = self.rng_for("latent-pressure", bucket, "", 0);
        let pressure_jitter = (unit_interval(&mut rng) - 0.5) * LATENT_JITTER_WIDTH;
        let burst_bad = self
            .burst_states
            .get(bucket as usize)
            .copied()
            .unwrap_or(false);
        let latent = LatentBucket {
            pressure_jitter,
            burst_bad,
        };
        self.latent_buckets.insert(bucket, latent);
        Ok(latent)
    }

    fn ensure_burst_states(&mut self, bucket: u32) -> Result<()> {
        while self.burst_states.len() <= bucket as usize {
            let current_bucket = self.burst_states.len() as u32;
            let previous_bad = self.burst_states.last().copied().unwrap_or(false);
            let pressure = self
                .manifest
                .randomization
                .pressure_at(f64::from(current_bucket));
            let bad = if let Some(burst_loss) = self
                .manifest
                .randomization
                .distribution_profile
                .as_ref()
                .and_then(|profile| profile.burst_loss.as_ref())
            {
                let probability = if previous_bad {
                    burst_loss.bad_to_good.evaluate(pressure)
                } else {
                    burst_loss.good_to_bad.evaluate(pressure)
                };
                let transitioned = sample_bernoulli(
                    probability,
                    self.rng_for("burst-loss", current_bucket, "", 0),
                )?;
                if previous_bad {
                    !transitioned
                } else {
                    transitioned
                }
            } else {
                false
            };

            self.burst_states.push(bad);
        }

        Ok(())
    }

    fn rng_for(&self, tag: &str, bucket: u32, worker_id: &str, sequence: u64) -> StdRng {
        let seed = derive_seed(self.seed, tag, bucket, worker_id, sequence);
        StdRng::seed_from_u64(seed)
    }
}

fn deterministic_sample(pressure: f64, sequence: u64) -> SampleOutcome {
    SampleOutcome {
        emitted: true,
        pressure,
        source_lag_ms: 8.0 + pressure * 64.0 + sequence as f64 * 0.2,
        collector_lag_ms: 28.0 + pressure * 88.0 + sequence as f64 * 0.3,
        received_lag_ms: None,
        burst_bad: false,
    }
}

fn sample_bernoulli(probability: f64, mut rng: StdRng) -> Result<bool> {
    let distribution = Bernoulli::new(probability.clamp(0.0, 1.0)).map_err(|error| {
        SimopsError::new(format!(
            "invalid Bernoulli probability {probability}: {error}"
        ))
    })?;
    Ok(distribution.sample(&mut rng))
}

fn sample_delay(delay: &DelayProfile, target_ms: f64, rng: &mut StdRng) -> Result<f64> {
    let target_ms = target_ms.max(0.0);
    if target_ms == 0.0 {
        return Ok(0.0);
    }

    match delay.distribution {
        DelayDistributionKind::Lognormal => {
            let distribution = LogNormal::new(target_ms.ln(), delay.sigma).map_err(|error| {
                SimopsError::new(format!("invalid lognormal delay profile: {error}"))
            })?;
            Ok(distribution.sample(rng))
        }
        DelayDistributionKind::Gamma => {
            let scale = target_ms / delay.shape;
            let distribution = Gamma::new(delay.shape, scale).map_err(|error| {
                SimopsError::new(format!("invalid gamma delay profile: {error}"))
            })?;
            Ok(distribution.sample(rng))
        }
    }
}

fn unit_interval(rng: &mut StdRng) -> f64 {
    rng.random::<f64>()
}

fn bucket_for_offset(offset_sec: f64) -> u32 {
    offset_sec.max(0.0).floor().min(f64::from(u32::MAX)) as u32
}

fn manifest_seed(manifest: &RunManifest) -> u64 {
    manifest.randomization.random_seed.unwrap_or_else(|| {
        let mut hash = FNV_OFFSET;
        hash = hash_bytes(hash, manifest.run_id.as_bytes());
        hash = hash_bytes(hash, format!("{:?}", manifest.scenario_id).as_bytes());
        hash
    })
}

fn derive_seed(seed: u64, tag: &str, bucket: u32, worker_id: &str, sequence: u64) -> u64 {
    let mut hash = FNV_OFFSET ^ seed;
    hash = hash_bytes(hash, tag.as_bytes());
    hash = hash_bytes(hash, &bucket.to_le_bytes());
    hash = hash_bytes(hash, worker_id.as_bytes());
    hash_bytes(hash, &sequence.to_le_bytes())
}

fn hash_bytes(mut hash: u64, bytes: &[u8]) -> u64 {
    for byte in bytes {
        hash ^= u64::from(*byte);
        hash = hash.wrapping_mul(FNV_PRIME);
    }
    hash
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;
    use crate::manifest::{RunManifest, WorkerKind};

    #[test]
    fn bernoulli_emission_is_deterministic_for_fixed_seed() {
        let manifest = stochastic_manifest(json!({
            "emission": {
                "probability": { "base": 0.5, "pressureSlope": 0.0, "min": 0.0, "max": 1.0 }
            }
        }));
        let worker = &manifest.workers[0];
        let mut left = ScenarioSampler::new(&manifest);
        let mut right = ScenarioSampler::new(&manifest);

        let first = left.sample(worker, 1, 0.0, 0.4).expect("left sample");
        let second = right.sample(worker, 1, 0.0, 0.4).expect("right sample");

        assert_eq!(first.emitted, second.emitted);
        assert_eq!(first.pressure, second.pressure);
    }

    #[test]
    fn shared_latent_pressure_moves_workers_together() {
        let manifest = stochastic_manifest(json!({
            "workerSensitivity": {
                "scheduler": 1.0,
                "storage": 1.0,
                "burst": 1.0,
                "fabric": 1.0
            }
        }));
        let scheduler = manifest
            .workers
            .iter()
            .find(|worker| worker.worker_kind == WorkerKind::Scheduler)
            .expect("scheduler worker");
        let storage = manifest
            .workers
            .iter()
            .find(|worker| worker.worker_kind == WorkerKind::Storage)
            .expect("storage worker");
        let mut sampler = ScenarioSampler::new(&manifest);

        let scheduler_sample = sampler
            .sample(scheduler, 1, 5.0, 0.5)
            .expect("scheduler sample");
        let storage_sample = sampler
            .sample(storage, 1, 5.0, 0.5)
            .expect("storage sample");

        assert!((scheduler_sample.pressure - storage_sample.pressure).abs() < 0.0001);
    }

    #[test]
    fn gilbert_elliott_bad_state_produces_adjacent_drops() {
        let manifest = stochastic_manifest(json!({
            "emission": {
                "probability": { "base": 1.0, "pressureSlope": 0.0, "min": 0.0, "max": 1.0 }
            },
            "burstLoss": {
                "goodToBad": { "base": 1.0, "pressureSlope": 0.0, "min": 0.0, "max": 1.0 },
                "badToGood": { "base": 0.0, "pressureSlope": 0.0, "min": 0.0, "max": 1.0 },
                "badEmitProbability": 0.0
            }
        }));
        let worker = &manifest.workers[0];
        let mut sampler = ScenarioSampler::new(&manifest);

        let first = sampler.sample(worker, 1, 0.0, 0.3).expect("first sample");
        let second = sampler.sample(worker, 2, 1.0, 0.3).expect("second sample");

        assert!(first.burst_bad);
        assert!(second.burst_bad);
        assert!(!first.emitted);
        assert!(!second.emitted);
    }

    fn stochastic_manifest(distribution_profile: serde_json::Value) -> RunManifest {
        let manifest = json!({
            "schemaVersion": "simops.run-manifest.v1",
            "runId": "RUN-HPC-TEST-001",
            "scenarioId": "scheduler-drift",
            "createdAt": "2026-07-04T18:00:00.000Z",
            "lifecycle": "created",
            "workbenchAnchor": {
                "jobId": "JOB-HPC-TEST",
                "evidencePackId": "EP-HPC-TEST",
                "gatewayEvidenceId": "SLURM-GATEWAY-TEST"
            },
            "runtimeLimitSec": 20,
            "transportBinding": "ndjson",
            "randomization": {
                "mode": "correlated-pressure",
                "randomSeed": 40401,
                "pressureCurve": [
                    { "offsetSec": 0, "pressure": 0.2 },
                    { "offsetSec": 20, "pressure": 0.7 }
                ],
                "baseline": [
                    { "metricPath": "scheduler.barrierWaitMs.p95", "nominalMin": 8, "nominalMax": 35, "unit": "ms" }
                ],
                "couplings": [],
                "events": [],
                "bounds": [
                    { "metricPath": "payload.barrierWaitMs.p95", "min": 0, "max": 1000, "unit": "ms" }
                ],
                "distributionProfile": distribution_profile
            },
            "workers": [
                {
                    "workerId": "scheduler-01",
                    "workerKind": "scheduler",
                    "payloadType": "schedulerCoScheduling",
                    "emitHz": 1,
                    "panelTarget": "Scheduler"
                },
                {
                    "workerId": "storage-01",
                    "workerKind": "storage",
                    "payloadType": "checkpointStorage",
                    "emitHz": 1,
                    "panelTarget": "Storage"
                }
            ],
            "artifacts": [
                {
                    "artifactId": "simops-run-manifest.test",
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

        RunManifest::from_json(&manifest.to_string()).expect("valid stochastic manifest")
    }
}
