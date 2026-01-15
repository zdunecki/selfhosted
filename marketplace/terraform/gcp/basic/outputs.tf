output "instance_id" {
  value = google_compute_instance.selfhosted.id
}

output "instance_name" {
  value = google_compute_instance.selfhosted.name
}

output "instance_zone" {
  value = google_compute_instance.selfhosted.zone
}

output "instance_ip" {
  value = google_compute_instance.selfhosted.network_interface[0].access_config[0].nat_ip
}

output "instance_internal_ip" {
  value = google_compute_instance.selfhosted.network_interface[0].network_ip
}
