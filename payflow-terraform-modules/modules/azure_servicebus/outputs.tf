output "namespace_id" {
  description = "Service Bus namespace resource ID."
  value       = azurerm_servicebus_namespace.this.id
}

output "namespace_name" {
  description = "Service Bus namespace name."
  value       = azurerm_servicebus_namespace.this.name
}

output "settlement_queue_name" {
  value = azurerm_servicebus_queue.settlement.name
}

output "webhook_queue_name" {
  value = azurerm_servicebus_queue.webhook.name
}

output "refund_queue_name" {
  value = azurerm_servicebus_queue.refund.name
}

output "primary_connection_string" {
  description = "Primary namespace connection string (store in Key Vault in production)."
  value       = azurerm_servicebus_namespace.this.default_primary_connection_string
  sensitive   = true
}
