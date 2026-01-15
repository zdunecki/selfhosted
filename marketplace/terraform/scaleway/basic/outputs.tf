output "server_id" {
  value = scaleway_instance_server.selfhosted.id
}

output "server_name" {
  value = scaleway_instance_server.selfhosted.name
}

output "server_zone" {
  value = scaleway_instance_server.selfhosted.zone
}

output "server_ip" {
  value = length(scaleway_instance_server.selfhosted.public_ips) > 0 ? scaleway_instance_server.selfhosted.public_ips[0].address : ""
}

output "server_ipv6" {
  value = "" # IPv6 not directly available via enable_dynamic_ip
}
