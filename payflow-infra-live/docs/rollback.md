# Rollback — PayFlow infrastructure and releases

**Origin:** R19, R18. **Related:** `docs/contracts/release-checklist.md`.

## What “rollback” means here

- **Infrastructure (Terraform):** revert to a **known-good Git commit** of `payflow-infra-live` (and pinned `payflow-terraform-modules` versions), run `terraform plan` against the target environment, and apply only after review. State remains in the remote backend; Terraform reconciles drift.
- **Application images:** revert Kubernetes **image digest/tag** in `payflow-platform-config` to the last healthy release; roll forward again after fix.
- **Emergency:** Azure Portal or CLI changes **without** Terraform are last resort; record them and **import** or reconcile into Terraform before closing an incident.

## Triggers (examples)

- Error rate or payment success SLO burn after an infra or app deploy (see runbooks when added).
- Failed `terraform apply` mid-run: fix partial state per Terraform guidance; do not leave unmanaged orphans undocumented.

## Steps (Terraform-first)

1. Identify last good **Git SHA** and module **version tags** (if modules are sourced by tag).
2. `git checkout <sha>` in a branch or revert PR.
3. `cd payflow-infra-live/envs/<dev|staging|prod>`.
4. `terraform init -reconfigure` (backend unchanged unless migrating).
5. `terraform plan -out=rollback.tfplan` — review with a second person for **prod**.
6. `terraform apply rollback.tfplan` — **prod** only with GitHub Environment approval if your pipeline enforces it.

## OIDC / pipeline rollback

- If a bad federated credential or role assignment broke CI: restore previous Entra **federated identity credential** or app registration settings; re-run plan from a clean runner with `permissions: id-token: write` intact (see GitHub docs on OIDC to Azure).

## Post-rollback validation

- Run environment smoke checklist from `docs/contracts/release-checklist.md`.
- Confirm `oidc_issuer_url` and database connectivity from a jump host or CI smoke job unchanged unless intentionally rolled back.

## Negative check (pipeline authors)

- A workflow that calls Azure **without** `permissions: id-token: write` (OIDC) where required will fail token minting. Fix workflow YAML before rotating Azure RBAC.
