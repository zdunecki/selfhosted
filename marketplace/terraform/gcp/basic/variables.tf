variable "project_id" {
  type        = string
  description = "GCP Project ID"
}

variable "name" {
  type        = string
  description = "Instance name"
}

variable "zone" {
  type        = string
  description = "GCP zone (e.g., europe-west1-a)"
}

variable "machine_type" {
  type        = string
  description = "Machine type (e.g., e2-medium)"
  default     = "e2-medium"
}

variable "image" {
  type        = string
  description = "Source image or image family"
  default     = "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts"
}

variable "disk_size_gb" {
  type        = number
  description = "Boot disk size in GB"
  default     = 25
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
