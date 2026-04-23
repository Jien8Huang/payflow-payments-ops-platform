# payflow-infra-live

## Purpose

**Live** Terraform **root modules** per environment under `envs/dev`, `envs/staging`, and `envs/prod`. Each root composes modules from `../../../payflow-terraform-modules/modules/*` (sibling repo folder at workspace root until git remotes are split).

## Environment differences (verification table)

| | **dev** | **staging** | **prod** |
|---|---------|-------------|----------|
| VNET CIDR (default in `terraform.tfvars.example`) | `10.42.0.0/16` | `10.43.0.0/16` | `10.44.0.0/16` |
| AKS nodes (`aks_node_count`) | 2 | 2 | 3 |
| AKS VM SKU (`terraform.tfvars.example`) | `Standard_B2s` | `Standard_B2s` | `Standard_D2s_v5` |
| Postgres SKU (example) | Burstable small | Burstable small | Burstable `B_Standard_B2ms` (tune for real prod) |
| Key Vault purge protection | `false` | `false` | `true` |
| Service Bus (queues `settlement`, `webhook`, `refund`) | yes | yes | yes |
| Remote state key (`backend.tf.example`) | `payflow-dev.tfstate` | `payflow-staging.tfstate` | `payflow-prod.tfstate` |

## Layout

```
payflow-infra-live/
  docs/
    rollback.md
    github-oidc-azure.md
  envs/
    dev/ staging/ prod/    # each: main.tf, variables.tf, versions.tf, providers.tf, outputs.tf
  test/
    .gitkeep               # future contract tests
```

## CI (Pattern A)

Workflow file: **`.github/workflows/terraform-plan.yml`** at the **repository root** (not under `payflow-infra-live/.github/`). PRs run `terraform fmt -check` and `terraform validate` for all three envs. Azure-backed `terraform plan` is **opt-in** via repo variable `RUN_AZURE_PLAN_IN_CI` — see `docs/github-oidc-azure.md`. When opt-in is enabled: **dev** plans on every workflow run that triggers the job; **staging** plans on pushes to `main` or manual `workflow_dispatch` with “run staging plan”; **prod** plans only via `workflow_dispatch` with “run prod plan” and the **`production`** GitHub Environment (required reviewers / protection rules apply there).

## First-time apply (human)

1. Create remote state storage (resource group + storage account + container) once per org.
2. Copy `envs/<env>/backend.tf.example` → `backend.tf`, fill names.
3. Copy `terraform.tfvars.example` → `terraform.tfvars`, set unique `name_prefix`.
4. `terraform init` → `terraform plan` → `terraform apply` with peer review for **prod**.

## Interfaces

| Upstream | Notes |
|----------|--------|
| `payflow-terraform-modules` | Module `source` uses relative path `../../../payflow-terraform-modules/modules/...` from each `envs/*` root. |
| `payflow-platform-config` | Consumes Terraform **outputs** (documented in `docs/contracts/release-checklist.md`). |

## Does not belong here

Application source; Kubernetes workload manifests (default: `payflow-platform-config`).
