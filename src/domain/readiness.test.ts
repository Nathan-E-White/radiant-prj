import { describe, expect, it } from "vitest";
import {
  buildEvidencePack,
  deploymentScore,
  diagnoseJob,
  fixtures,
  hashArtifact,
  requirementCoverage,
  runFleetTelemetryToy,
  runPassiveThermalToy,
  runTransportToy,
  validateTraceability
} from "./readiness";

describe("synthetic analysis kernels", () => {
  it("runs a deterministic transport-style sweep", () => {
    const result = runTransportToy({
      cells: 12,
      sourceStrength: 1,
      absorption: 0.16,
      scatter: 0.62
    });

    expect(result.scalarFlux).toHaveLength(12);
    expect(result.peakScalarFlux).toBeGreaterThan(2);
    expect(result.edgeLeakageProxy).toBeGreaterThan(0);
    expect(result.balanceResidual).toBeLessThan(0.5);
    expect(result.iterations).toBe(28);
  });

  it("computes passive thermal toy margin", () => {
    const result = runPassiveThermalToy({
      heatKw: 950,
      ambientC: 42,
      thermalResistanceCPerKw: 0.18,
      limitC: 260
    });

    expect(result.peakTemperatureC).toBe(213);
    expect(result.marginC).toBe(47);
    expect(result.status).toBe("margin-positive");
  });

  it("flags fleet telemetry anomalies deterministically", () => {
    const result = runFleetTelemetryToy({
      values: [100, 101, 99, 100, 102, 98, 120, 101, 100],
      zLimit: 2.2,
      packetLossPct: 0.7,
      missingPacketLimitPct: 1.5
    });

    expect(result.channelsFlagged).toBe(1);
    expect(result.maxZScore).toBeGreaterThan(2.2);
    expect(result.status).toBe("review-required");
  });
});

describe("evidence discipline", () => {
  it("validates fixture traceability", () => {
    expect(validateTraceability()).toEqual([]);
  });

  it("diagnoses the HPC module drift failure", () => {
    const failedJob = fixtures.computeJobs.find((job) => job.id === "JOB-HPC-404");
    expect(failedJob).toBeDefined();
    const diagnosis = diagnoseJob(failedJob!);

    expect(diagnosis.rootCause.toLowerCase()).toContain("worker");
    expect(diagnosis.preventativeControl.toLowerCase()).toContain("module");
  });

  it("generates stable artifact hashes and evidence packs", () => {
    const job = fixtures.computeJobs[0];
    const firstHash = hashArtifact({ artifact: "flux-profile.csv", jobId: job.id });
    const secondHash = hashArtifact({ jobId: job.id, artifact: "flux-profile.csv" });
    const pack = buildEvidencePack(job);

    expect(firstHash).toBe(secondHash);
    expect(pack.requirementIds).toEqual(job.linkedRequirements);
    expect(Object.keys(pack.artifactHashes)).toEqual(job.artifacts);
  });

  it("reports requirement coverage and deployment readiness", () => {
    const coverage = requirementCoverage();

    expect(coverage.every((record) => record.status === "verified")).toBe(true);
    expect(deploymentScore()).toBeGreaterThanOrEqual(70);
  });
});
