variable "name" {
  type = string
}

variable "region" {
  type = string
}

variable "size" {
  type = string
}

variable "image" {
  type = string
}

variable "ssh_public_key" {
  type = string
}

variable "ssh_fingerprint" {
  type        = string
  description = "MD5 fingerprint of the SSH public key (e.g., ab:cd:ef:...)"
}

variable "tags" {
  type    = list(string)
  default = []
}

variable "volume_size" {
  type        = number
  description = "Size of the separate data volume in GiB"
  default     = 100
}
