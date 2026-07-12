ARG SIMOPS_GO_BUILDER_IMAGE=golang:1.26-alpine
ARG SIMOPS_GATEWAY_RUNTIME_IMAGE=alpine:3.21

FROM ${SIMOPS_GO_BUILDER_IMAGE} AS builder-base

WORKDIR /src
COPY backend/slurm-gateway/go.mod backend/slurm-gateway/go.mod
COPY backend/slurm-gateway backend/slurm-gateway

WORKDIR /src/backend/slurm-gateway

FROM builder-base AS gateway-builder
RUN CGO_ENABLED=0 GOOS=linux go build -tags dataplane -trimpath -ldflags="-s -w" -o /out/slurm-gateway ./cmd/server

FROM builder-base AS builder
RUN go test -tags dataplane,iceberggo ./...
RUN CGO_ENABLED=0 GOOS=linux go build -tags dataplane,iceberggo -trimpath -ldflags="-s -w" -o /out/slurm-gateway ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -tags dataplane -trimpath -ldflags="-s -w" -o /out/simops-stream-gateway ./cmd/simops-stream-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -tags dataplane -trimpath -ldflags="-s -w" -o /out/simops-timescale-writer ./cmd/simops-timescale-writer
RUN CGO_ENABLED=0 GOOS=linux go build -tags dataplane,iceberggo -trimpath -ldflags="-s -w" -o /out/simops-iceberg-writer ./cmd/simops-iceberg-writer
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/simops-webtransport-probe ./cmd/simops-webtransport-probe
RUN CGO_ENABLED=0 GOOS=linux go build -tags dataplane -trimpath -ldflags="-s -w" -o /out/workbench-projection-writer ./cmd/workbench-projection-writer
RUN CGO_ENABLED=0 GOOS=linux go build -tags dataplane -trimpath -ldflags="-s -w" -o /out/twin-projector ./cmd/twin-projector
RUN CGO_ENABLED=0 GOOS=linux go build -tags dataplane,iceberggo -trimpath -ldflags="-s -w" -o /out/workbench-iceberg-writer ./cmd/workbench-iceberg-writer

FROM ${SIMOPS_GATEWAY_RUNTIME_IMAGE} AS runtime-base

RUN apk add --no-cache docker-cli && addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

ENV SLURM_GATEWAY_ADDR=:8080
ENV SLURM_GATEWAY_MODE=mock

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

FROM runtime-base AS gateway-runtime
COPY --from=gateway-builder /out/slurm-gateway /app/slurm-gateway
RUN mkdir -p /app/.local/slurm-scripts && chown -R appuser:appgroup /app

USER appuser

CMD ["/app/slurm-gateway"]

FROM runtime-base AS full-runtime
COPY --from=builder /out/slurm-gateway /app/slurm-gateway
COPY --from=builder /out/simops-stream-gateway /app/simops-stream-gateway
COPY --from=builder /out/simops-timescale-writer /app/simops-timescale-writer
COPY --from=builder /out/simops-iceberg-writer /app/simops-iceberg-writer
COPY --from=builder /out/simops-webtransport-probe /app/simops-webtransport-probe
COPY --from=builder /out/workbench-projection-writer /app/workbench-projection-writer
COPY --from=builder /out/twin-projector /app/twin-projector
COPY --from=builder /out/workbench-iceberg-writer /app/workbench-iceberg-writer
RUN mkdir -p /app/.local/slurm-scripts && chown -R appuser:appgroup /app

USER appuser

CMD ["/app/slurm-gateway"]
