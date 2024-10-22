variable "name_prefix" {
  description = "Prefix for AKS cluster name (Azure limit: cluster name length)."
  type        = string
}

variable "location" {
  description = "Azure region (must match resource group)."
  type        = string
}

variable "resource_group_name" {
  description = "Existing resource group name (from network module or shared RG)."
  type        = string
}

variable "aks_subnet_id" {
  description = "Subnet ID for the default node pool (Azure CNI)."
  type        = string
}

variable "kubernetes_version" {
  description = "AKS control plane version; null uses the current default for the channel."
  type        = string
  default     = null
}

variable "node_vm_size" {
  description = "VM SKU for system node pool."
  type        = string
  default     = "Standard_B2s"
}

variable "node_count" {
  description = "Initial node count for the default pool (portfolio dev default)."
  type        = number
  default     = 2
}

variable "tags" {
  description = "Tags applied to the cluster."
  type        = map(string)
  default     = {}
}
