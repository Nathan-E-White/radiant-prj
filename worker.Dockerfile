ARG MOCK_WORKER_BUN_IMAGE=oven/bun:1.3.14

FROM ${MOCK_WORKER_BUN_IMAGE}
WORKDIR /worker
COPY scripts/mock-worker.mjs scripts/mock-worker.mjs
COPY src/data/readiness-fixtures.json src/data/readiness-fixtures.json
CMD ["bun", "scripts/mock-worker.mjs"]
