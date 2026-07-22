variable "SIMOPS_GO_BUILDER_IMAGE" {
  default = "golang:1.26-alpine"
}

variable "CONSOLE_BUN_IMAGE" {
  default = "oven/bun:1.3.14"
}

variable "CONSOLE_NGINX_IMAGE" {
  default = "nginx:1.27-alpine"
}

variable "MOCK_WORKER_BUN_IMAGE" {
  default = "oven/bun:1.3.14"
}

variable "SIMOPS_GATEWAY_RUNTIME_IMAGE" {
  default = "alpine:3.21"
}

variable "SCADA_RUST_BUILDER_IMAGE" {
  default = "rust:1-alpine"
}

variable "SCADA_STANDINS_RUNTIME_IMAGE" {
  default = "gcr.io/distroless/static-debian13:nonroot"
}

variable "SIMOPS_RUST_BUILDER_IMAGE" {
  default = "rust:1-alpine"
}

variable "SIMOPS_GENERATOR_RUNTIME_IMAGE" {
  default = "gcr.io/distroless/static-debian13:nonroot"
}

group "packaging" {
  targets = [
    "console",
    "mock-worker",
    "reactor-telemetry-worker",
    "simops-generator",
    "slurm-gateway",
    "simops-moq-gateway",
    "simops-webtransport-probe",
    "simops-timescale-writer",
    "simops-iceberg-writer",
    "workbench-projection-writer",
    "twin-projector",
    "workbench-iceberg-writer",
  ]
}

target "_packaging-common" {
  context = "."
}

target "console" {
  inherits = ["_packaging-common"]
  dockerfile = "Dockerfile"
  tags = ["radiant-packaging-console:verify"]
  args = {
    CONSOLE_BUN_IMAGE = "${CONSOLE_BUN_IMAGE}"
    CONSOLE_NGINX_IMAGE = "${CONSOLE_NGINX_IMAGE}"
  }
}

target "mock-worker" {
  inherits = ["_packaging-common"]
  dockerfile = "worker.Dockerfile"
  tags = ["radiant-packaging-mock-worker:verify"]
  args = {
    MOCK_WORKER_BUN_IMAGE = "${MOCK_WORKER_BUN_IMAGE}"
  }
}

target "reactor-telemetry-worker" {
  inherits = ["_packaging-common"]
  dockerfile = "deploy/scada-standins.Dockerfile"
  tags = ["radiant-packaging-reactor-telemetry-worker:verify"]
  args = {
    SCADA_RUST_BUILDER_IMAGE = "${SCADA_RUST_BUILDER_IMAGE}"
    SCADA_STANDINS_RUNTIME_IMAGE = "${SCADA_STANDINS_RUNTIME_IMAGE}"
  }
}

target "simops-generator" {
  inherits = ["_packaging-common"]
  dockerfile = "deploy/simops-generator.Dockerfile"
  tags = ["radiant-packaging-simops-generator:verify"]
  args = {
    SIMOPS_RUST_BUILDER_IMAGE = "${SIMOPS_RUST_BUILDER_IMAGE}"
    SIMOPS_GENERATOR_RUNTIME_IMAGE = "${SIMOPS_GENERATOR_RUNTIME_IMAGE}"
  }
}

target "_go-runtime" {
  inherits = ["_packaging-common"]
  dockerfile = "deploy/slurm-gateway.Dockerfile"
  args = {
    SIMOPS_GO_BUILDER_IMAGE = "${SIMOPS_GO_BUILDER_IMAGE}"
    SIMOPS_GATEWAY_RUNTIME_IMAGE = "${SIMOPS_GATEWAY_RUNTIME_IMAGE}"
  }
}

target "slurm-gateway" {
  inherits = ["_go-runtime"]
  target = "gateway-runtime"
  tags = ["radiant-packaging-slurm-gateway:verify"]
}

target "simops-moq-gateway" {
  inherits = ["_go-runtime"]
  target = "simops-stream-gateway-runtime"
  tags = ["radiant-packaging-simops-moq-gateway:verify"]
}

target "simops-webtransport-probe" {
  inherits = ["_go-runtime"]
  target = "simops-webtransport-probe-runtime"
  tags = ["radiant-packaging-simops-webtransport-probe:verify"]
}

target "simops-timescale-writer" {
  inherits = ["_go-runtime"]
  target = "simops-timescale-writer-runtime"
  tags = ["radiant-packaging-simops-timescale-writer:verify"]
}

target "simops-iceberg-writer" {
  inherits = ["_go-runtime"]
  target = "simops-iceberg-writer-runtime"
  tags = ["radiant-packaging-simops-iceberg-writer:verify"]
}

target "workbench-projection-writer" {
  inherits = ["_go-runtime"]
  target = "workbench-projection-writer-runtime"
  tags = ["radiant-packaging-workbench-projection-writer:verify"]
}

target "twin-projector" {
  inherits = ["_go-runtime"]
  target = "twin-projector-runtime"
  tags = ["radiant-packaging-twin-projector:verify"]
}

target "workbench-iceberg-writer" {
  inherits = ["_go-runtime"]
  target = "workbench-iceberg-writer-runtime"
  tags = ["radiant-packaging-workbench-iceberg-writer:verify"]
}
