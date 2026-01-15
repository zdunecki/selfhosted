terraform {
  required_providers {
    scaleway = {
      source  = "scaleway/scaleway"
      version = "~> 2.40"
    }
  }
}

provider "scaleway" {
  zone = var.zone
}

# Create instance
resource "scaleway_instance_server" "selfhosted" {
  name              = var.name
  type              = var.commercial_type
  image             = var.image_id
  project_id        = var.project_id
  enable_dynamic_ip = true
  tags              = concat([var.name], var.tags)

  # Cloud-init user data for SSH key
  user_data = var.ssh_public_key != "" ? {
    cloud-init = <<-EOF
      #cloud-config
      ssh_authorized_keys:
        - ${var.ssh_public_key}
      EOF
  } : {}
}
