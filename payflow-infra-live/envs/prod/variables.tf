variable "name_prefix" {
  description = "Unique prefix for Azure resource names in this environment."
  type        = string
}

variable "environment_name" {
  description = "Environment label for tagging."
  type        = string
  default     = "prod"
}

variable "location" {
  type    = string
  default = "westeurope"
}

variable "address_space" {
  type = list(string)
}

variable "aks_subnet_cidr" {
  type = string
}

variable "postgres_subnet_cidr" {
  type = string
}

variable "aks_node_count" {
  type    = number
  default = 2
}

variable "aks_vm_size" {
  type    = string
  default = "Standard_B2s"
}

variable "postgres_sku" {
  type    = string
  default = "B_Standard_B1ms"
}

variable "key_vault_purge_protection" {
  type    = bool
  default = false
}

variable "tags" {
  type    = map(string)
  default = {}
}
