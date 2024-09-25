output "resource_group_name" {
  description = "Resource group containing the VNET and subnets."
  value       = azurerm_resource_group.this.name
}

output "resource_group_id" {
  description = "Resource group ID."
  value       = azurerm_resource_group.this.id
}

output "location" {
  description = "Azure region."
  value       = azurerm_resource_group.this.location
}

output "virtual_network_id" {
  description = "VNET resource ID."
  value       = azurerm_virtual_network.this.id
}

output "virtual_network_name" {
  description = "VNET name."
  value       = azurerm_virtual_network.this.name
}

output "aks_subnet_id" {
  description = "Subnet ID for AKS node pool (Azure CNI)."
  value       = azurerm_subnet.aks.id
}

output "postgres_subnet_id" {
  description = "Delegated subnet ID for PostgreSQL Flexible Server VNet integration."
  value       = azurerm_subnet.postgres.id
}
