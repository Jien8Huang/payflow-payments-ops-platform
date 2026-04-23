# payflow-terraform-modules

## Purpose

Reusable **Terraform** modules for **Microsoft Azure**: **VNET** topology, **AKS** (workload-identity-ready), managed **PostgreSQL Flexible Server**, **Key Vault** access patterns, and (later) observability baseline hooks. Modules are **environment-agnostic**; callers pass sizing and names via variables.

## v1 scope note (object storage)

Requirements **R16** mentions object storage **where the narrative needs it**. For **v1** of this portfolio, **Blob / Storage Account modules are intentionally omitted** — artifacts are container images and PostgreSQL data. If audit exports or report blobs are added later, introduce a dedicated module and document the retention story.

## Interfaces to sibling repos

| Consumer | Relationship |
| --- | --- |
| `payflow-infra-live` | Imports modules via **version-pinned** source (git tag or registry path). Consumes **outputs** (subnet IDs, AKS OIDC issuer URL, Postgres FQDN, Key Vault URI). |

## Hiring signals addressed

IaC **module boundaries**, **versioned** platform abstractions, **blast-radius** separation from live `terraform apply`.

## Security notes

- No secrets committed; use **Key Vault** references and CI **OIDC** for automation per plan.
- Document **Cilium vs Azure NPM** / network policy engine choice in module READMEs when networking modules land.

## Modules

| Module | Path |
|--------|------|
| VNET + subnets (AKS + delegated Postgres) | `modules/azure_network/` |
| AKS (workload identity + OIDC issuer) | `modules/azure_aks/` |
| PostgreSQL Flexible (VNet + private DNS) | `modules/azure_postgres/` |
| Key Vault (RBAC-enabled) | `modules/azure_keyvault/` |
| Service Bus namespace + settlement/webhook/refund queues | `modules/azure_servicebus/` |

## Minimal example

See `examples/minimal/` — wired stack for `terraform validate` / `terraform plan` (requires Azure auth for plan).

Local validation without apply:

```bash
./scripts/validate.sh
```

## Does not belong here

Environment-specific `*.tfvars`, remote **backend** configuration, or production secret values.
