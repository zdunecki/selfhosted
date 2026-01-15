variable "name" {
  type        = string
  description = "Server name/title"
}

variable "hostname" {
  type        = string
  description = "Server hostname"
}

variable "zone" {
  type        = string
  description = "UpCloud zone (e.g., de-fra1, fi-hel2)"
}

variable "plan" {
  type        = string
  description = "UpCloud plan (e.g., 1xCPU-1GB)"
  default     = ""
}

variable "template_name" {
  type        = string
  description = "Template name to use (e.g., 'Ubuntu Server 24.04 LTS (Noble Numbat)')"
}

variable "disk_size_gb" {
  type        = number
  description = "Disk size in GB"
  default     = 25
}

variable "disk_tier" {
  type        = string
  description = "Storage tier (maxiops or hdd)"
  default     = "maxiops"
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
