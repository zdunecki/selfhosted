terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.40"
    }
  }
}

provider "digitalocean" {}

# Look up existing SSH key by fingerprint (passed from Go)
data "digitalocean_ssh_keys" "all" {}

locals {
  existing_key = [for k in data.digitalocean_ssh_keys.all.ssh_keys : k if k.fingerprint == var.ssh_fingerprint]
  key_exists   = length(local.existing_key) > 0
  ssh_key_id   = local.key_exists ? local.existing_key[0].id : digitalocean_ssh_key.selfhosted[0].id
}

resource "digitalocean_ssh_key" "selfhosted" {
  count      = local.key_exists ? 0 : 1
  name       = "${var.name}-key"
  public_key = var.ssh_public_key
}

resource "digitalocean_droplet" "selfhosted" {
  name       = var.name
  region     = var.region
  size       = var.size
  image      = var.image
  ssh_keys   = [local.ssh_key_id]
  tags       = var.tags
  backups    = false
  monitoring = true
}
