terraform {
  required_providers {
    vultr = {
      source  = "vultr/vultr"
      version = "~> 2.0"
    }
  }
}

provider "vultr" {
  # API key is provided via VULTR_API_KEY environment variable
}

# Create instance
resource "vultr_instance" "selfhosted" {
  label    = var.name
  hostname = var.hostname
  region   = var.region
  plan     = var.plan
  os_id    = var.os_id

  # SSH keys
  ssh_key_ids = var.ssh_key_ids

  # Tags
  tags = concat([var.name], var.tags)

  # Disable IPv6 (optional, can be enabled if needed)
  enable_ipv6 = false
}
