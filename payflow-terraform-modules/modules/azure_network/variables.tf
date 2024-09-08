variable "name_prefix" {
  description = "Short prefix for resource names (letters and digits, unique in subscription)."
  type        = string
}

variable "location" {
  description = "Azure region (default aligns with plan: EU)."
  type        = string
  default     = "westeurope"
}

variable "address_space" {
  description = "VNET address space CIDR."
  type        = list(string)
  default     = ["10.42.0.0/16"]
}

variable "aks_subnet_cidr" {
  description = "Subnet CIDR for AKS nodes (must fit in address_space)."
  type        = string
  default     = "10.42.1.0/24"
}

variable "postgres_subnet_cidr" {
  description = "Dedicated subnet for PostgreSQL Flexible Server VNet integration (delegated)."
  type        = string
  default     = "10.42.2.0/24"
}

variable "tags" {
  description = "Tags applied to all resources in this module."
  type        = map(string)
  default     = {}
}
