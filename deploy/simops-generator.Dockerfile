ARG SIMOPS_RUST_BUILDER_IMAGE=rust:1-alpine
ARG SIMOPS_GENERATOR_RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot

FROM ${SIMOPS_RUST_BUILDER_IMAGE} AS builder

WORKDIR /src
COPY workers/simops-generator/Cargo.toml workers/simops-generator/Cargo.lock ./workers/simops-generator/
COPY workers/simops-generator/src ./workers/simops-generator/src
COPY workers/simops-generator/tests ./workers/simops-generator/tests
COPY examples/simulation-ops ./examples/simulation-ops

WORKDIR /src/workers/simops-generator
RUN cargo test --locked
RUN cargo build --locked --release

FROM ${SIMOPS_GENERATOR_RUNTIME_IMAGE}

COPY --from=builder /src/workers/simops-generator/target/release/simops-generator /simops-generator
COPY --from=builder /src/examples/simulation-ops /examples/simulation-ops

ENTRYPOINT ["/simops-generator"]
