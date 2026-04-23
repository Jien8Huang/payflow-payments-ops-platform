variable "name_prefix" {
  description = "Prefix for the Service Bus namespace name (must be globally unique with suffix)."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group for the namespace."
  type        = string
}

variable "sku" {
  description = "Service Bus SKU (Basic has no topics; Standard+ required for duplicate detection features)."
  type        = string
  default     = "Standard"
}

variable "max_delivery_count" {
  description = "Max deliveries before dead-lettering."
  type        = number
  default     = 10
}

variable "tags" {
  description = "Resource tags."
  type        = map(string)
  default     = {}
}
