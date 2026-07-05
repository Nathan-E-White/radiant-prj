FROM rust:1-alpine AS builder

WORKDIR /src
COPY workers/simops-generator/Cargo.toml workers/simops-generator/Cargo.lock ./workers/simops-generator/
COPY workers/simops-generator/src ./workers/simops-generator/src
COPY workers/simops-generator/tests ./workers/simops-generator/tests
COPY examples/simulation-ops ./examples/simulation-ops

WORKDIR /src/workers/simops-generator
RUN cargo test --locked
RUN RUSTFLAGS="-C target-feature=+crt-static" cargo build --locked --release

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /src/workers/simops-generator/target/release/simops-generator /simops-generator
COPY --from=builder /src/examples/simulation-ops /examples/simulation-ops

ENTRYPOINT ["/simops-generator"]
