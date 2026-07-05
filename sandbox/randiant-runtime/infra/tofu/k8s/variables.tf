variable "kubeconfig_path" {
  type        = string
  description = "Path to kubeconfig for target cluster"
}

variable "namespace" {
  type        = string
  description = "Kubernetes namespace where the pod should be created"
  default     = "default"
}

variable "container_image" {
  type        = string
  description = "Container image for toy pod"
  default     = "alpine:latest"
}

variable "app_label" {
  type        = string
  description = "Pod label used for readiness lookup and logs"
  default     = "orchestrator-alpine"
}

variable "pod_name" {
  type        = string
  description = "Name of the ephemeral echo pod"
  default     = "orchestrator-alpine-echo"
}

variable "echo_message" {
  type        = string
  description = "Message to echo from the pod"
  default     = "hello from kubernetes"
}
