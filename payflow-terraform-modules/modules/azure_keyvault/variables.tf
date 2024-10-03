variable "key_vault_name" {
  description = "Globally unique Key Vault name (3-24 alphanumeric)."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group for the vault."
  type        = string
}

variable "sku_name" {
  description = "Key Vault SKU."
  type        = string
  default     = "standard"
}

variable "purge_protection_enabled" {
  description = "Enable purge protection (use true in prod)."
  type        = bool
  default     = false
}

variable "tags" {
  type    = map(string)
  default = {}
}
