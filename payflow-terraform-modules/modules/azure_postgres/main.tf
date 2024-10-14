locals {
  dns_zone_name = coalesce(
    var.private_dns_zone_name,
    "${replace(var.name_prefix, "-", "")}.private.postgres.database.azure.com"
  )
}

resource "azurerm_private_dns_zone" "postgres" {
  name                = local.dns_zone_name
  resource_group_name = var.resource_group_name
  tags                = var.tags
}

resource "azurerm_private_dns_zone_virtual_network_link" "postgres" {
  name                  = "${var.name_prefix}-pg-dns-link"
  resource_group_name   = var.resource_group_name
  private_dns_zone_name = azurerm_private_dns_zone.postgres.name
  virtual_network_id    = var.virtual_network_id
  registration_enabled  = false
  tags                  = var.tags
}

resource "azurerm_postgresql_flexible_server" "this" {
  name                   = "${var.name_prefix}-psql"
  resource_group_name    = var.resource_group_name
  location               = var.location
  version                = var.postgres_version
  delegated_subnet_id    = var.delegated_subnet_id
  private_dns_zone_id    = azurerm_private_dns_zone.postgres.id
  administrator_login    = var.administrator_login
  administrator_password = var.administrator_password
  sku_name                 = var.sku_name
  storage_mb               = 32768
  backup_retention_days    = 7
  geo_redundant_backup_enabled = false
  tags                     = var.tags

  depends_on = [azurerm_private_dns_zone_virtual_network_link.postgres]
}
