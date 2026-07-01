terraform {
  required_version = ">= 1.5.0"
}

locals {
  environment_name = var.environment_name

  network = {
    cidr                = "10.42.0.0/24"
    scheduler_subnet    = "10.42.0.0/28"
    worker_subnet       = "10.42.0.32/27"
    artifact_subnet     = "10.42.0.96/28"
    monitoring_subnet   = "10.42.0.128/28"
    allowed_ingress_tcp = [22, 443, 6817, 6818]
  }

  scheduler = {
    hostname      = "readiness-scheduler-01"
    instance_hint = "c7i.large"
    services      = ["slurmctld", "munge", "module-index-exporter"]
    file_systems  = ["/hpc/artifacts", "/hpc/modules", "/hpc/scratch"]
    role_tags     = ["scheduler", "control-plane", "dry-run"]
  }

  worker_pool = {
    desired_count = var.worker_count
    instance_hint = "c7i.xlarge"
    services      = ["slurmd", "munge", "node-exporter"]
    modules       = ["radlab_transport/0.4", "radlab_thermal/0.2", "openmpi/4.1"]
    role_tags     = ["worker", "compute", "dry-run"]
  }

  artifact_bucket = {
    name             = "${local.environment_name}-objective-evidence"
    retention_days   = 30
    hash_manifest    = true
    immutable_record = false
    role_tags        = ["artifact-store", "dry-run"]
  }

  monitoring_endpoint = {
    hostname       = "readiness-monitor-01"
    scrape_targets = ["scheduler", "worker_pool", "artifact_bucket"]
    alert_mode     = "dry-run"
    role_tags      = ["monitoring", "dry-run"]
  }

  security_groups = {
    scheduler_to_worker = {
      from = "scheduler"
      to   = "worker_pool"
      tcp  = [6817, 6818]
    }
    worker_to_artifact_store = {
      from = "worker_pool"
      to   = "artifact_bucket"
      tcp  = [443]
    }
    operator_to_scheduler = {
      from = "operator"
      to   = "scheduler"
      tcp  = [22]
    }
  }
}
