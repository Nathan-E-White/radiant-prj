output "compute_environment" {
  description = "Dry-run declaration of the scheduler, worker, artifact, monitoring, network, and security posture."
  value = {
    environment_name    = local.environment_name
    network             = local.network
    scheduler           = local.scheduler
    worker_pool         = local.worker_pool
    artifact_bucket     = local.artifact_bucket
    monitoring_endpoint = local.monitoring_endpoint
    security_groups     = local.security_groups
  }
}

output "deployment_note" {
  description = "Safety boundary for the Terraform demo."
  value       = "This module declares infrastructure intent only; it provisions no real cloud, site, reactor, or HPC resources."
}
