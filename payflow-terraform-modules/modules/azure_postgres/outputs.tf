output "server_id" {
  description = "PostgreSQL Flexible Server resource ID."
  value       = azurerm_postgresql_flexible_server.this.id
}

output "server_fqdn" {
  description = "FQDN for PostgreSQL (private hostname when using VNet integration)."
  value       = azurerm_postgresql_flexible_server.this.fqdn
}

output "server_name" {
  description = "Server name."
  value       = azurerm_postgresql_flexible_server.this.name
}

output "private_dns_zone_id" {
  description = "Private DNS zone ID."
  value       = azurerm_private_dns_zone.postgres.id
}
