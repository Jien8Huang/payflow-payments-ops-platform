variable "name_prefix" {
  description = "Short unique prefix for all resources (letters, digits, hyphens)."
  type        = string

  validation {
    condition     = length(var.name_prefix) >= 3 && length(var.name_prefix) <= 20
    error_message = "name_prefix must be between 3 and 20 characters for Key Vault naming constraints in this example."
  }
}

variable "location" {
  description = "Azure region."
  type        = string
  default     = "westeurope"
}
