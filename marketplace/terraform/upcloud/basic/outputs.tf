output "server_id" {
  value = upcloud_server.selfhosted.id
}

output "server_name" {
  value = upcloud_server.selfhosted.title
}

output "server_zone" {
  value = upcloud_server.selfhosted.zone
}

output "server_ip" {
  # Get the public IP from the second network interface (index 1, which is the public interface)
  value = length(upcloud_server.selfhosted.network_interface) > 1 ? upcloud_server.selfhosted.network_interface[1].ip_address : ""
}
