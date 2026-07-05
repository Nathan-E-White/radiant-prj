

terraform {
  required_providers {
    customdocker = {
      # Points directly to offline filesystem mirror def(s)
      source = "registry.opentofu.org/local/customdocker"
      version = "1.0.0"
    }
  }
}

provider "customdocker" {}

resource "customdocker_container" "app_server" {
  # TODO(This is a placeholder)
  image = "nginx:alpine"
  name  = "opentofu_managed_container"

  logging {
    # TODO(This is a placeholder)
    driver = "json-file"
  }

  networks {
    # TODO(This is a placeholder)
    network_id = "bridge"
  }
}