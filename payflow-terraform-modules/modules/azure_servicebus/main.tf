locals {
  # Namespace: 6–50 chars, alphanumeric + hyphen; start with letter, end with letter or digit.
  namespace_name = substr("${var.name_prefix}-sb", 0, 50)
}

resource "azurerm_servicebus_namespace" "this" {
  name                = local.namespace_name
  location            = var.location
  resource_group_name = var.resource_group_name
  sku                 = var.sku
  tags                = var.tags
}

resource "azurerm_servicebus_queue" "settlement" {
  name         = "settlement"
  namespace_id = azurerm_servicebus_namespace.this.id

  max_delivery_count                   = var.max_delivery_count
  dead_lettering_on_message_expiration = true
}

resource "azurerm_servicebus_queue" "webhook" {
  name         = "webhook"
  namespace_id = azurerm_servicebus_namespace.this.id

  max_delivery_count                   = var.max_delivery_count
  dead_lettering_on_message_expiration = true
}

resource "azurerm_servicebus_queue" "refund" {
  name         = "refund"
  namespace_id = azurerm_servicebus_namespace.this.id

  max_delivery_count                   = var.max_delivery_count
  dead_lettering_on_message_expiration = true
}
