output "droplet_id" {
  value = digitalocean_droplet.selfhosted.id
}

output "droplet_ipv4" {
  value = digitalocean_droplet.selfhosted.ipv4_address
}
