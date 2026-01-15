variable "name" {
  type        = string
  description = "Instance name"
}

variable "zone" {
  type        = string
  description = "Scaleway zone (e.g., fr-par-1)"
}

variable "commercial_type" {
  type        = string
  description = "Commercial type (e.g., DEV1-S)"
  default     = "DEV1-S"
}

variable "image_id" {
  type        = string
  description = "Image ID or label (e.g., ubuntu_jammy, ubuntu_noble, or zone/uuid format)"
}

variable "project_id" {
  type        = string
  description = "Scaleway project ID"
}

variable "ssh_public_key" {
  type        = string
  description = "SSH public key to add to the instance"
  default     = ""
}

variable "tags" {
  type        = list(string)
  description = "Additional tags"
  default     = []
}
