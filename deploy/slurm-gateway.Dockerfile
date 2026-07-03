FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY backend/slurm-gateway/go.mod backend/slurm-gateway/go.mod
COPY backend/slurm-gateway backend/slurm-gateway

WORKDIR /src/backend/slurm-gateway
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/slurm-gateway ./cmd/server

FROM alpine:3.21

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /out/slurm-gateway /app/slurm-gateway
RUN mkdir -p /app/.local/slurm-scripts && chown -R appuser:appgroup /app

USER appuser

ENV SLURM_GATEWAY_ADDR=:8080
ENV SLURM_GATEWAY_MODE=mock

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/slurm-gateway"]
