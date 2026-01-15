output "instance_id" {
  value = vultr_instance.selfhosted.id
}

output "instance_label" {
  value = vultr_instance.selfhosted.label
}

output "instance_region" {
  value = vultr_instance.selfhosted.region
}

output "instance_ip" {
  value = vultr_instance.selfhosted.main_ip
}

output "instance_status" {
  value = vultr_instance.selfhosted.power_status
}
