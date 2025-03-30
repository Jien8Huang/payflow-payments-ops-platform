resource "random_password" "postgres" {
  length           = 32
  special          = true
  override_special = "!#%&*-_=+"
  min_lower        = 4
  min_upper        = 4
  min_numeric      = 4
}

resource "random_id" "kv" {
  byte_length = 3
}

locals {
  tags = merge(var.tags, {
    Environment = var.environment_name
    ManagedBy   = "terraform"
    Project     = "payflow"
  })
  kv_raw  = "pf${replace(var.name_prefix, "-", "")}${random_id.kv.hex}"
  kv_name = length(local.kv_raw) > 24 ? substr(local.kv_raw, 0, 24) : local.kv_raw
}

module "network" {
  source              = "../../../payflow-terraform-modules/modules/azure_network"
  name_prefix         = var.name_prefix
  location            = var.location
  address_space       = var.address_space
  aks_subnet_cidr     = var.aks_subnet_cidr
  postgres_subnet_cidr = var.postgres_subnet_cidr
  tags                = local.tags
}

module "keyvault" {
  source                   = "../../../payflow-terraform-modules/modules/azure_keyvault"
  key_vault_name           = local.kv_name
  location                 = module.network.location
  resource_group_name      = module.network.resource_group_name
  purge_protection_enabled = var.key_vault_purge_protection
  tags                     = local.tags
}

module "postgres" {
  source                  = "../../../payflow-terraform-modules/modules/azure_postgres"
  name_prefix             = var.name_prefix
  location                = module.network.location
  resource_group_name     = module.network.resource_group_name
  delegated_subnet_id     = module.network.postgres_subnet_id
  virtual_network_id      = module.network.virtual_network_id
  administrator_password  = random_password.postgres.result
  sku_name                = var.postgres_sku
  tags                    = local.tags
}

module "aks" {
  source              = "../../../payflow-terraform-modules/modules/azure_aks"
  name_prefix         = var.name_prefix
  location            = module.network.location
  resource_group_name = module.network.resource_group_name
  aks_subnet_id       = module.network.aks_subnet_id
  node_count          = var.aks_node_count
  node_vm_size        = var.aks_vm_size
  tags                = local.tags
}
