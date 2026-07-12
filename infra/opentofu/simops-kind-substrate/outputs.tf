output "namespace" {
  description = "Namespace consumed by the Kubernetes runtime adapter."
  value       = kubernetes_namespace_v1.simops.metadata[0].name
}

output "gateway_service_account" {
  description = "Service account whose RBAC allows Job lifecycle operations."
  value       = kubernetes_service_account_v1.gateway.metadata[0].name
}

output "worker_service_account" {
  description = "Service account assigned to ordinary simulation workers."
  value       = kubernetes_service_account_v1.worker.metadata[0].name
}

output "runtime_config_map" {
  description = "ConfigMap that can be consumed by the gateway Deployment."
  value       = kubernetes_config_map_v1.runtime_adapter.metadata[0].name
}

output "runtime_adapter_env" {
  description = "Values matching the Go Kubernetes adapter environment contract."
  value       = kubernetes_config_map_v1.runtime_adapter.data
}
