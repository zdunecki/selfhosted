terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {}

# Enable Compute API
resource "google_project_service" "compute" {
  project = var.project_id
  service = "compute.googleapis.com"

  disable_on_destroy = false
}

# Firewall rule for SSH
resource "google_compute_firewall" "allow_ssh" {
  name    = "${var.name}-allow-ssh"
  network = "default"
  project = var.project_id

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = [var.name]

  depends_on = [google_project_service.compute]
}

# Firewall rule for HTTP/HTTPS (needed for ACME HTTP-01)
resource "google_compute_firewall" "allow_http_https" {
  name    = "${var.name}-allow-http-https"
  network = "default"
  project = var.project_id

  allow {
    protocol = "tcp"
    ports    = ["80", "443"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = [var.name]

  depends_on = [google_project_service.compute]
}

# Startup script to add SSH key
locals {
  ssh_key_encoded = var.ssh_public_key != "" ? base64encode(var.ssh_public_key) : ""
  startup_script = <<-EOF
    #!/bin/bash
    set -euo pipefail
    
    mkdir -p /root/.ssh
    chmod 700 /root/.ssh
    touch /root/.ssh/authorized_keys
    chmod 600 /root/.ssh/authorized_keys
    
    %{ if var.ssh_public_key != "" ~}
    SSH_KEY=$(echo '${local.ssh_key_encoded}' | base64 -d)
    grep -qF "$SSH_KEY" /root/.ssh/authorized_keys || echo "$SSH_KEY" >> /root/.ssh/authorized_keys
    %{ endif ~}
    
    if [ -f /etc/ssh/sshd_config ]; then
      sed -i.bak 's/^#\?PermitRootLogin.*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config || true
      systemctl restart ssh || systemctl restart sshd || true
    fi
  EOF
}

# Compute instance
resource "google_compute_instance" "selfhosted" {
  name         = var.name
  machine_type = var.machine_type
  zone         = var.zone
  project      = var.project_id

  tags = concat([var.name], var.tags)

  boot_disk {
    initialize_params {
      image = var.image
      size  = var.disk_size_gb
    }
    auto_delete = true
  }

  network_interface {
    network = "default"

    access_config {
      // Ephemeral public IP
    }
  }

  metadata = {
    startup-script = local.startup_script
  }

  depends_on = [
    google_project_service.compute,
    google_compute_firewall.allow_ssh,
    google_compute_firewall.allow_http_https,
  ]
}
