ARG CONSOLE_BUN_IMAGE=oven/bun:1.3.14
ARG CONSOLE_NGINX_IMAGE=nginx:1.27-alpine

FROM ${CONSOLE_BUN_IMAGE} AS deps
WORKDIR /app
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

FROM deps AS build
COPY . .
RUN bun run build

FROM ${CONSOLE_NGINX_IMAGE}
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD wget -qO- http://127.0.0.1/ >/dev/null || exit 1
