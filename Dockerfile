FROM oven/bun:1.3.14 AS deps
WORKDIR /app
COPY package.json bun.lockb* ./
RUN bun install

FROM deps AS build
COPY . .
RUN bun run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD wget -qO- http://127.0.0.1/ >/dev/null || exit 1
