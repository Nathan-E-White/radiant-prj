variable "environment_name" {
  description = "Dry-run environment name used to label generated infrastructure intent."
  type        = string
  default     = "kaleidos-readiness"
}

variable "worker_count" {
  description = "Desired worker count for the mock hybrid compute pool."
  type        = number
  default     = 3

  validation {
    condition     = var.worker_count >= 1 && var.worker_count <= 8
    error_message = "worker_count must stay in the toy demo range of 1 to 8."
  }
}
