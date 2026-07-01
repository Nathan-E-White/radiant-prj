FROM oven/bun:1.3.14
WORKDIR /worker
COPY package.json bun.lockb* ./
RUN bun install
COPY scripts ./scripts
COPY src/data ./src/data
CMD ["bun", "run", "mock:worker"]
