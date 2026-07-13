locals {
  common_labels = {
    "app.kubernetes.io/name"       = "radiant-simops"
    "app.kubernetes.io/component"  = "runtime-substrate"
    "app.kubernetes.io/managed-by" = "opentofu"
  }
}

resource "kubernetes_namespace_v1" "simops" {
  metadata {
    name   = var.namespace
    labels = local.common_labels
  }
}

resource "kubernetes_service_account_v1" "gateway" {
  metadata {
    name      = var.gateway_service_account
    namespace = kubernetes_namespace_v1.simops.metadata[0].name
    labels    = local.common_labels
  }
  automount_service_account_token = true
}

resource "kubernetes_service_account_v1" "worker" {
  metadata {
    name      = var.worker_service_account
    namespace = kubernetes_namespace_v1.simops.metadata[0].name
    labels    = local.common_labels
  }
  automount_service_account_token = false
}

resource "kubernetes_role_v1" "runtime" {
  metadata {
    name      = "simops-runtime"
    namespace = kubernetes_namespace_v1.simops.metadata[0].name
    labels    = local.common_labels
  }

  rule {
    api_groups = ["batch"]
    resources  = ["jobs"]
    verbs      = ["create", "get", "list", "watch", "delete"]
  }

  rule {
    api_groups = [""]
    resources  = ["pods"]
    verbs      = ["get", "list", "watch"]
  }
}

resource "kubernetes_role_binding_v1" "runtime" {
  metadata {
    name      = "simops-runtime"
    namespace = kubernetes_namespace_v1.simops.metadata[0].name
    labels    = local.common_labels
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = kubernetes_role_v1.runtime.metadata[0].name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account_v1.gateway.metadata[0].name
    namespace = kubernetes_namespace_v1.simops.metadata[0].name
  }
}

resource "kubernetes_config_map_v1" "runtime_adapter" {
  metadata {
    name      = "simops-runtime-adapter"
    namespace = kubernetes_namespace_v1.simops.metadata[0].name
    labels    = local.common_labels
  }

  data = {
    SIMOPS_WORKER_RUNTIME                    = "kubernetes"
    SIMOPS_WORKER_KUBERNETES_NAMESPACE       = kubernetes_namespace_v1.simops.metadata[0].name
    SIMOPS_WORKER_KUBERNETES_SERVICE_ACCOUNT = kubernetes_service_account_v1.worker.metadata[0].name
    SIMOPS_WORKER_CLEANUP_TTL                = var.worker_cleanup_ttl
  }
}
