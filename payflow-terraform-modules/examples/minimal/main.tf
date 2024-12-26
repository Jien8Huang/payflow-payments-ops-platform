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
  # Key Vault name: 3-24 chars, alphanumeric only (hyphens allowed in KV names actually - Azure allows hyphen)
  kv_raw = "pf${replace(var.name_prefix, "-", "")}${random_id.kv.hex}"
  kv_name = length(local.kv_raw) > 24 ? substr(local.kv_raw, 0, 24) : local.kv_raw
}

module "network" {
  source      = "../../modules/azure_network"
  name_prefix = var.name_prefix
  location    = var.location
}

module "keyvault" {
  source                  = "../../modules/azure_keyvault"
  key_vault_name          = local.kv_name
  location                = module.network.location
  resource_group_name     = module.network.resource_group_name
  purge_protection_enabled = false
}

module "postgres" {
  source                   = "../../modules/azure_postgres"
  name_prefix              = var.name_prefix
  location                 = module.network.location
  resource_group_name      = module.network.resource_group_name
  delegated_subnet_id      = module.network.postgres_subnet_id
  virtual_network_id       = module.network.virtual_network_id
  administrator_password   = random_password.postgres.result
}

module "aks" {
  source              = "../../modules/azure_aks"
  name_prefix         = var.name_prefix
  location            = module.network.location
  resource_group_name = module.network.resource_group_name
  aks_subnet_id       = module.network.aks_subnet_id
}
