terraform {
  required_version = ">= 1.6.0"
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.34"
    }
  }
}

provider "kubernetes" {
  config_path = var.kubeconfig_path
}

resource "kubernetes_pod_v1" "orchestrator_echo" {
  metadata {
    name      = var.pod_name
    namespace = var.namespace
    labels = {
      app = var.app_label
    }
  }

  spec {
    restart_policy = "Never"

    container {
      name    = "echo"
      image   = var.container_image
      command = ["/bin/sh", "-c", "echo \"$ECHO_MESSAGE\" && sleep 5"]
      env {
        name  = "ECHO_MESSAGE"
        value = var.echo_message
      }
    }
  }
}

output "pod_selector" {
  value = "app=${var.app_label}"
}

output "pod_name" {
  value = kubernetes_pod_v1.orchestrator_echo.metadata[0].name
}
