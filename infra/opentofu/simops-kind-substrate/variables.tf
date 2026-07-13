variable "namespace" {
  description = "Namespace containing the SimOps gateway and run-scoped worker Jobs."
  type        = string
  default     = "radiant-simops"
}

variable "gateway_service_account" {
  description = "Service account used by the Go control plane to manage worker Jobs."
  type        = string
  default     = "simops-gateway"
}

variable "worker_service_account" {
  description = "Credential-minimal service account assigned to ordinary worker Pods."
  type        = string
  default     = "simops-worker"
}

variable "worker_cleanup_ttl" {
  description = "Default successful-Job TTL exposed to the Kubernetes runtime adapter."
  type        = string
  default     = "60s"

  validation {
    condition     = can(regex("^([0-9]+(ns|us|µs|ms|s|m|h))+$", var.worker_cleanup_ttl))
    error_message = "worker_cleanup_ttl must use Go duration units such as 60s, 10m, or 1h30m."
  }
}

variable "kubeconfig_path" {
  description = "Kubeconfig used by OpenTofu for substrate planning or provisioning."
  type        = string
  default     = null
  nullable    = true
}

variable "kubeconfig_context" {
  description = "Optional kubeconfig context, such as kind-radiant-simops-smoke."
  type        = string
  default     = null
  nullable    = true
}
