# Docker Packaging Verification

Issue #126 measures the repository input, project-owned runtime images, representative builder cache, and production browser assets. The executable budgets live in `config/docker-packaging-budgets.json`; `bun run docker:packaging:verify` writes the machine-readable result.

## Before and after

| Measurement | Before | After | Change |
| --- | ---: | ---: | ---: |
| Root Docker context | 502.34 MB | 7.99 MB | -98.4% |
| Console image | 24.06 MB | 24.06 MB | unchanged multi-stage runtime |
| Mock-worker image | 160.77 MB | 87.00 MB | -45.9% and no application dependency tree |
| Shared Go `full-runtime` | 197.06 MB per role | replaced | eight narrow targets |
| Gateway image | 197.06 MB shared baseline | 33.44 MB | -83.0%; retains Docker CLI |
| MoQ gateway image | 197.06 MB shared baseline | 8.04 MB | -95.9% |
| WebTransport probe image | 197.06 MB shared baseline | 6.79 MB | -96.6%; narrow one-shot probe target |
| Timescale writer image | 197.06 MB shared baseline | 8.12 MB | -95.9% |
| SimOps Iceberg writer image | 197.06 MB shared baseline | 25.63 MB | -87.0% |
| Workbench projection writer image | 197.06 MB shared baseline | 8.14 MB | -95.9% |
| Twin projector image | 197.06 MB shared baseline | 8.25 MB | -95.8% |
| Workbench Iceberg writer image | 197.06 MB shared baseline | 25.54 MB | -87.0% |
| Browser assets | 3.54 MB raw / 2.24 MB gzip | 3.54 MB raw / 2.24 MB gzip | exposed and budgeted |

The pre-change worker and shared Go runtime were reconstructed from the original Dockerfile instructions with the same source, tags, linker flags, and runtime packages. Go build concurrency and cache mounts were bounded so the measurement could finish in the 4 GB local OrbStack environment; these controls do not change final binaries.

## Builder cache evidence

The initial concurrent baseline attempt ended with both BuildKit sessions reporting `rpc error: code = Unavailable ... EOF`. It left 1.827 GB aggregate cache, of which 443.1 MB was reclaimable. A later bounded repeat, after the same 4 GB engine had accumulated the baseline and verification images, failed with `no space left on device`. Removing only the named issue-126 test images restored the engine; no volume or broad cache pruning was used.

The passing representative post-change run serialized images, limited Go package parallelism to two, and used explicit Go module/build cache mounts. It measured:

- 2.537 GB cache growth;
- 2.890 GB aggregate cache;
- 2.618 GB reclaimable cache.

All remain below the committed 4 GiB growth, 6 GiB aggregate, and 4 GiB reclaimable budgets. The verifier reports cache pressure; it does not prune it.

An earlier complete repeat using stable verification tags measured zero additional cache growth, confirming warm-builder repeatability. The committed machine-readable evidence is the final expanded ten-role run, including the dedicated WebTransport probe.

## Runtime content proof

The verifier runs the mock worker and asserts that `/worker` has no `node_modules`, `package.json`, or Bun lockfile. It starts an inspection container for every Go role, asserts exactly one file under `/app`, and requires Docker CLI only in the Slurm gateway image.

Final-image budgets allow roughly 25–31% headroom above the measured arm64 outputs. GitHub's amd64 runner uses separate ceilings with roughly 26–38% headroom because Docker's unpacked image size is architecture-dependent. The exact amd64 observations and provenance from Actions run `29680581898` are preserved in `docs/verification/docker-packaging-evidence-amd64.json`; the packaging contract asserts every amd64 ceiling remains within 20–40% of its recorded output. The mock-worker dependency-tree regression is also rejected structurally by inspecting the image for `node_modules`, `package.json`, and lockfiles, rather than relying on an architecture-sensitive size proxy alone.

The committed arm64 machine-readable run is `docs/verification/docker-packaging-evidence.json`; the amd64 runner snapshot is `docs/verification/docker-packaging-evidence-amd64.json`. CI regenerates the verifier schema under `.local/docker-packaging-evidence/latest.json` and uploads it even when a budget fails.
