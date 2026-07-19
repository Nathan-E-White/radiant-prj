ARG SIMOPS_GO_BUILDER_IMAGE=golang:1.26-alpine
ARG SIMOPS_GATEWAY_RUNTIME_IMAGE=alpine:3.21

FROM ${SIMOPS_GO_BUILDER_IMAGE} AS builder-base

WORKDIR /src
COPY backend/slurm-gateway/go.mod backend/slurm-gateway/go.mod
COPY backend/slurm-gateway backend/slurm-gateway

WORKDIR /src/backend/slurm-gateway

FROM builder-base AS test
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go test -p 2 -tags dataplane,iceberggo ./...

FROM builder-base AS gateway-builder
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -p 2 -tags dataplane -trimpath -ldflags="-s -w" -o /out/slurm-gateway ./cmd/server

FROM builder-base AS simops-stream-gateway-builder
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -p 2 -tags dataplane -trimpath -ldflags="-s -w" -o /out/simops-stream-gateway ./cmd/simops-stream-gateway

FROM builder-base AS simops-timescale-writer-builder
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -p 2 -tags dataplane -trimpath -ldflags="-s -w" -o /out/simops-timescale-writer ./cmd/simops-timescale-writer

FROM builder-base AS simops-iceberg-writer-builder
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -p 2 -tags dataplane,iceberggo -trimpath -ldflags="-s -w" -o /out/simops-iceberg-writer ./cmd/simops-iceberg-writer

FROM builder-base AS simops-webtransport-probe-builder
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -p 2 -trimpath -ldflags="-s -w" -o /out/simops-webtransport-probe ./cmd/simops-webtransport-probe

FROM builder-base AS workbench-projection-writer-builder
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -p 2 -tags dataplane -trimpath -ldflags="-s -w" -o /out/workbench-projection-writer ./cmd/workbench-projection-writer

FROM builder-base AS twin-projector-builder
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -p 2 -tags dataplane -trimpath -ldflags="-s -w" -o /out/twin-projector ./cmd/twin-projector

FROM builder-base AS workbench-iceberg-writer-builder
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -p 2 -tags dataplane,iceberggo -trimpath -ldflags="-s -w" -o /out/workbench-iceberg-writer ./cmd/workbench-iceberg-writer

FROM ${SIMOPS_GATEWAY_RUNTIME_IMAGE} AS runtime-base

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

FROM runtime-base AS gateway-runtime-base

RUN apk add --no-cache docker-cli

ENV SLURM_GATEWAY_ADDR=:8080
ENV SLURM_GATEWAY_MODE=mock

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

FROM gateway-runtime-base AS gateway-runtime
COPY --from=gateway-builder /out/slurm-gateway /app/slurm-gateway
RUN mkdir -p /app/.local/slurm-scripts && chown -R appuser:appgroup /app

USER appuser

CMD ["/app/slurm-gateway"]

FROM runtime-base AS simops-stream-gateway-runtime
COPY --from=simops-stream-gateway-builder /out/simops-stream-gateway /app/simops-stream-gateway
USER appuser
CMD ["/app/simops-stream-gateway"]

FROM runtime-base AS simops-timescale-writer-runtime
COPY --from=simops-timescale-writer-builder /out/simops-timescale-writer /app/simops-timescale-writer
USER appuser
CMD ["/app/simops-timescale-writer"]

FROM runtime-base AS simops-iceberg-writer-runtime
COPY --from=simops-iceberg-writer-builder /out/simops-iceberg-writer /app/simops-iceberg-writer
USER appuser
CMD ["/app/simops-iceberg-writer"]

FROM runtime-base AS simops-webtransport-probe-runtime
COPY --from=simops-webtransport-probe-builder /out/simops-webtransport-probe /app/simops-webtransport-probe
USER appuser
ENTRYPOINT ["/app/simops-webtransport-probe"]

FROM runtime-base AS workbench-projection-writer-runtime
COPY --from=workbench-projection-writer-builder /out/workbench-projection-writer /app/workbench-projection-writer
USER appuser
CMD ["/app/workbench-projection-writer"]

FROM runtime-base AS twin-projector-runtime
COPY --from=twin-projector-builder /out/twin-projector /app/twin-projector
USER appuser
CMD ["/app/twin-projector"]

FROM runtime-base AS workbench-iceberg-writer-runtime
COPY --from=workbench-iceberg-writer-builder /out/workbench-iceberg-writer /app/workbench-iceberg-writer
USER appuser
CMD ["/app/workbench-iceberg-writer"]
