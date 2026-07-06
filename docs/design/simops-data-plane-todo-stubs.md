# SimOps Data Plane Gate Closure

These gates tracked the remaining integration work after the Redpanda-backed crash slice. They stay here as the current closure record for the pre-workbench implementation gate. No manifest-only or debug-endpoint cosplay.

## CLOSED(SIMOPS-ICEBERG-FINISH)

Iceberg warehouse proof is implemented around `backend/slurm-gateway/internal/gateway/simops_iceberg_go_writer.go`.

Closure evidence:

- `SIMOPS_ICEBERG_WRITER_MODE=iceberg-go` appends worker telemetry into `simops.telemetry_frames`.
- The writer reloads the table from the SQL catalog after append, plans data-file scan tasks, reads rows back through Iceberg-Go, and compares Redpanda topic/partition/offset coordinates before allowing commit.
- Artifact status remains `prepared` until the append and fresh readback succeed; failures mark the artifact `failed`.
- `scripts/simops-local-smoke.sh` still requires Parquet files in MinIO and the committed artifact state, so catalog metadata alone is not accepted.

## CLOSED(SIMOPS-MOQ-WEBTRANSPORT-LINK)

The live telemetry link is implemented as a WebTransport session in `backend/slurm-gateway/cmd/simops-stream-gateway` using `github.com/quic-go/webtransport-go`. The payload envelope preserves the selected namespace and tracks: `lifecycle`, `workers/{worker_id}/telemetry`, `workers/{worker_id}/quality`, and `artifacts`.

Closure evidence:

- WebTransport clients connect to `https://127.0.0.1:9443/moq/simops` over HTTP/3/QUIC; plain HTTP requests receive an explicit upgrade-required response with the local certificate fingerprint endpoint.
- `simops-moq-gateway` counts actual WebTransport sessions in `subscriber_count`.
- The debug track snapshot remains inspection-only.
- `simops-webtransport-probe` subscribes over WebTransport and requires at least one telemetry and one quality message for the smoke run.
- This implementation uses the Go WebTransport stack with a MoQ-compatible SimOps envelope. It does not claim full moq-dev relay/CDN semantics; adopting `moq-dev/moq` remains a future compatibility expansion if the product needs the broader `moq-lite` stack.

## CLOSED(SIMOPS-DOCKER-PREFLIGHT)

The Docker registry lookup and content-pull failure modes in `scripts/simops-local-smoke.sh` are hardened.

Closure evidence:

- The smoke script runs bounded preflights for required image metadata and image content before `docker compose build`.
- The failure messages distinguish registry/base-image lookup or pull stalls from SimOps service failures.
- Base images and service images can be overridden with `SIMOPS_*_IMAGE` variables for pinned digests or local mirrors, and `SIMOPS_SMOKE_IMAGE_CACHE_ONLY=1` supports a preloaded image cache.
- The Compose-local worker launch path uses the `radiant-simops-local` network and a bounded two-frame worker override so the smoke checks data-plane integration instead of stress volume.
- Once the preflights pass, the smoke proceeds to Timescale, WebTransport, and Iceberg data-plane assertions.
