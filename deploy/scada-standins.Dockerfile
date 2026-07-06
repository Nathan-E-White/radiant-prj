ARG SCADA_RUST_BUILDER_IMAGE=rust:1-alpine
ARG SCADA_STANDINS_RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot

FROM ${SCADA_RUST_BUILDER_IMAGE} AS builder

WORKDIR /src
COPY workers/scada-standins/Cargo.toml workers/scada-standins/Cargo.lock ./workers/scada-standins/
COPY workers/scada-standins/src ./workers/scada-standins/src
COPY workers/scada-standins/tests ./workers/scada-standins/tests

WORKDIR /src/workers/scada-standins
RUN cargo test --locked
RUN cargo build --locked --release

FROM ${SCADA_STANDINS_RUNTIME_IMAGE}

COPY --from=builder /src/workers/scada-standins/target/release/scada-standins /scada-standins

ENTRYPOINT ["/scada-standins"]
