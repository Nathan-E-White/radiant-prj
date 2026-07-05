output "pod_selector" {
  value = "app=${var.app_label}"
}

output "pod_name" {
  value = kubernetes_pod_v1.orchestrator_echo.metadata[0].name
}
