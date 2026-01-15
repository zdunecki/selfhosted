output "droplet_id" {
  value = digitalocean_droplet.selfhosted.id
}

output "droplet_ipv4" {
  value = digitalocean_droplet.selfhosted.ipv4_address
}

output "volume_id" {
  value = digitalocean_volume.data.id
}

output "volume_name" {
  value = digitalocean_volume.data.name
}

output "volume_size" {
  value = digitalocean_volume.data.size
}
