output "cluster_id" {
  description = "AKS cluster resource ID."
  value       = azurerm_kubernetes_cluster.this.id
}

output "cluster_name" {
  description = "AKS cluster name."
  value       = azurerm_kubernetes_cluster.this.name
}

output "kube_config_raw" {
  description = "Admin kubeconfig (sensitive). Prefer workload identity + az aks get-credentials in CI for human-less flows."
  value       = azurerm_kubernetes_cluster.this.kube_config_raw
  sensitive   = true
}

output "oidc_issuer_url" {
  description = "OIDC issuer URL for workload identity federation (Entra)."
  value       = azurerm_kubernetes_cluster.this.oidc_issuer_url
}

output "node_resource_group" {
  description = "Managed resource group containing node VMs."
  value       = azurerm_kubernetes_cluster.this.node_resource_group
}

output "kubelet_identity" {
  description = "Kubelet managed identity (object id) for ACR pull or other integrations."
  value       = azurerm_kubernetes_cluster.this.kubelet_identity[0].object_id
}
