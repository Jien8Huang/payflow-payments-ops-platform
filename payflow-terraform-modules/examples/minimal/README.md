# Minimal wired example

Wires **`azure_network`**, **`azure_keyvault`**, **`azure_postgres`**, and **`azure_aks`** for a single-region PayFlow-style footprint.

## Prerequisites

- Terraform **>= 1.5**
- Azure subscription + authenticated `azurerm` provider (`az login` or OIDC in CI)
- Sufficient quota for AKS, PostgreSQL Flexible, and Key Vault in the chosen region

## Usage

```bash
cd payflow-terraform-modules/examples/minimal
cp terraform.tfvars.example terraform.tfvars   # edit name_prefix
terraform init -upgrade
terraform fmt -recursive
terraform validate
terraform plan -out=tfplan
```

`terraform plan` / `apply` require Azure credentials; `terraform validate` requires `terraform init` only.

## Outputs contract

Downstream **`payflow-infra-live`** roots should consume the same logical outputs (names may differ by composition):

| Output | Consumer |
|--------|----------|
| `resource_group_name` | Platform docs, CI |
| `aks_subnet_id` | AKS module / cluster profile |
| `postgres_subnet_id` | Postgres module |
| `virtual_network_id` | Private DNS links, peering |
| `oidc_issuer_url` | Entra federated credentials for workload identity |
| `postgres_fqdn` | App `DATABASE_URL` host (with creds from Key Vault, not from example outputs in prod) |
| `key_vault_uri` | CSI / app config |

Do **not** commit `terraform.tfvars`, `*.tfstate`, or `*.tfplan`.
