terraform {
  required_providers {
    upcloud = {
      source  = "UpCloudLtd/upcloud"
      version = "~> 5.0"
    }
  }
}

provider "upcloud" {
  # Credentials are provided via environment variables:
  # UPCLOUD_TOKEN (preferred) or UPCLOUD_USERNAME and UPCLOUD_PASSWORD
  # The provider will use token if UPCLOUD_TOKEN is set, otherwise username/password
}

# Create server
resource "upcloud_server" "selfhosted" {
  hostname = var.hostname
  title    = var.name
  zone     = var.zone
  plan     = var.plan

  # Firewall must be enabled in trial mode
  firewall = true

  # Network interfaces
  network_interface {
    type = "utility"
    ip_address_family = "IPv4"
  }

  network_interface {
    type = "public"
    ip_address_family = "IPv4"
  }

  # Login user with SSH key
  login {
    user = "root"
    create_password = false
    keys = var.ssh_public_key != "" ? [var.ssh_public_key] : []
  }

  # Template - use template name
  # Note: The disk name may have a "terraform-" prefix from Terraform's resource naming
  template {
    storage = var.template_name
    size    = var.disk_size_gb
    title   = "${var.name}-disk"
  }

  # Tags
  tags = concat([var.name], var.tags)

  # Metadata
  metadata = true
}
