# Module: `azure_postgres`

## Purpose

Deploys **Azure Database for PostgreSQL Flexible Server** with **VNet integration** (delegated subnet + **private DNS zone** + VNet link). Matches PayFlow requirement for managed PostgreSQL (**R16**).

## Inputs / outputs

See `variables.tf` / `outputs.tf`.

| Name | Data classification |
|------|---------------------|
| `administrator_password` | **Secret** — never commit; inject via CI/Key Vault in live roots. |
| `server_fqdn` | Non-secret hostname (private DNS in integrated mode). |

## Constraints

- `delegated_subnet_id` must use a subnet delegated to `Microsoft.DBforPostgreSQL/flexibleServers` (see `azure_network`).
- Private DNS zone name must end with **`.private.postgres.database.azure.com`** for this integration pattern.
