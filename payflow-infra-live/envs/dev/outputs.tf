output "resource_group_name" {
  description = "Network / platform resource group."
  value       = module.network.resource_group_name
}

output "aks_subnet_id" {
  value = module.network.aks_subnet_id
}

output "postgres_subnet_id" {
  value = module.network.postgres_subnet_id
}

output "virtual_network_id" {
  value = module.network.virtual_network_id
}

output "oidc_issuer_url" {
  description = "AKS OIDC issuer for Entra workload federation."
  value       = module.aks.oidc_issuer_url
}

output "postgres_fqdn" {
  value = module.postgres.server_fqdn
}

output "key_vault_uri" {
  value = module.keyvault.vault_uri
}

output "aks_cluster_name" {
  value = module.aks.cluster_name
}
