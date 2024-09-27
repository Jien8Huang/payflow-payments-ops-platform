# Module: `azure_keyvault`

## Purpose

Creates an **Azure Key Vault** with **Azure RBAC** authorization (`rbac_authorization_enabled = true`). Role assignments for Terraform, AKS workload identity, and human operators are **not** created inside this module — grant `Key Vault Secrets User` / `Key Vault Administrator` at `payflow-infra-live` or via pipeline to least-privilege principals.

## Outputs (data classification)

| Output | Classification |
|--------|----------------|
| `vault_uri` | Non-secret public DNS name for the vault. |
| Secrets inside the vault | **Secret** — never in Terraform state for real passwords in prod without careful design. |

## Notes

- `key_vault_name` must be **globally unique** and 3–24 characters `[a-zA-Z0-9-]`.
- `purge_protection_enabled` defaults to **false** for portfolio dev; set **true** in production roots when policy requires it.
