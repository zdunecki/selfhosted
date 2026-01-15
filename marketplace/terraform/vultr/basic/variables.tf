variable "name" {
  type        = string
  description = "Instance label/name"
}

variable "hostname" {
  type        = string
  description = "Instance hostname"
}

variable "region" {
  type        = string
  description = "Vultr region (e.g., ewr, ams, sgp)"
}

variable "plan" {
  type        = string
  description = "Vultr plan ID (e.g., vc2-1c-1gb)"
}

variable "os_id" {
  type        = number
  description = "OS ID for Ubuntu"
}

variable "ssh_key_ids" {
  type        = list(string)
  description = "List of SSH key IDs to attach"
  default     = []
}

variable "tags" {
  type        = list(string)
  description = "Additional tags"
  default     = []
}
