variable "name_prefix" {
  description = "Prefix for PostgreSQL server name (Azure naming rules apply)."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group for the server and DNS zone."
  type        = string
}

variable "delegated_subnet_id" {
  description = "Subnet ID delegated to Microsoft.DBforPostgreSQL/flexibleServers."
  type        = string
}

variable "virtual_network_id" {
  description = "VNET ID for private DNS zone virtual network link."
  type        = string
}

variable "administrator_login" {
  description = "PostgreSQL admin username."
  type        = string
  default     = "payflowadmin"
}

variable "administrator_password" {
  description = "PostgreSQL admin password (use random or Key Vault reference in live roots)."
  type        = string
  sensitive   = true
}

variable "sku_name" {
  description = "Flexible server SKU (e.g. B_Standard_B1ms for dev)."
  type        = string
  default     = "B_Standard_B1ms"
}

variable "postgres_version" {
  description = "PostgreSQL major version."
  type        = string
  default     = "16"
}

variable "private_dns_zone_name" {
  description = "Private DNS zone name; must end with .private.postgres.database.azure.com when using VNet integration."
  type        = string
  default     = null
}

variable "tags" {
  type    = map(string)
  default = {}
}
