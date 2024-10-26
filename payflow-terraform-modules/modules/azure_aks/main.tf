resource "azurerm_kubernetes_cluster" "this" {
  name                = "${var.name_prefix}-aks"
  location            = var.location
  resource_group_name = var.resource_group_name
  dns_prefix          = "${var.name_prefix}aks"
  kubernetes_version  = var.kubernetes_version

  workload_identity_enabled = true
  oidc_issuer_enabled         = true

  default_node_pool {
    name            = "system"
    vm_size         = var.node_vm_size
    node_count      = var.node_count
    vnet_subnet_id  = var.aks_subnet_id
    os_disk_size_gb = 60
  }

  identity {
    type = "SystemAssigned"
  }

  network_profile {
    network_plugin = "azure"
    dns_service_ip = "10.200.0.10"
    service_cidr   = "10.200.0.0/16"
  }

  tags = var.tags
}
