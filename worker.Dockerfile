FROM oven/bun:1.3.14
WORKDIR /worker
COPY scripts/mock-worker.mjs scripts/mock-worker.mjs
COPY src/data/readiness-fixtures.json src/data/readiness-fixtures.json
CMD ["bun", "scripts/mock-worker.mjs"]
