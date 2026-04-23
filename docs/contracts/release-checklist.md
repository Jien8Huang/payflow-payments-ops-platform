# Release and promotion checklist (PayFlow)

**Origin:** R17–R19, R25. **Plan:** `docs/plans/payflow-platform-plan.md`.

## CI workflow layout (normative — Pattern A)

**LOCKED:** **Pattern A — repo-root workflows only.**

- All GitHub Actions workflows live under **`.github/workflows/`** at the **repository root** (this workspace root today).
- Jobs use `paths` / `paths-ignore` (and optional `workflow_dispatch`) so that:
  - changes under `payflow-app/**` trigger application CI,
  - changes under `payflow-terraform-modules/**` trigger module validation,
  - changes under `payflow-infra-live/**` trigger Terraform plan/apply workflows,
  - changes under `payflow-platform-config/**` trigger manifest lint / `kustomize build` checks (local: `scripts/ci/validate-platform-config.sh`; optional `kubeconform` when installed). YAML syntax under `payflow-platform-config/**`: `scripts/ci/lint-yaml.sh` (requires `python3` and `PyYAML`).
- **No** duplicate workflow trees under `payflow-*/.github/` for this repository. If repositories are split into separate git remotes later, each remote may adopt its own `.github/workflows/` at **that** root — this document applies to **this** monorepo-style workspace until split.

**OIDC:** Terraform and deploy jobs that touch Azure must use **OIDC** (`permissions: id-token: write`) per Microsoft Learn and GitHub hardening docs referenced in the plan.

## Promotion order

1. **Database migrations** (if any) compatible with **currently running** app version (additive-first rule in early phases).
2. **Terraform apply** to **staging** (plan reviewed) before app rollout when infra outputs change secrets or URLs consumed by the app.
3. **Container image** build, sign/attest if used, push to registry with **immutable digest** recorded.
4. **`payflow-platform-config` overlay** updates image digest/tag for **staging**; deploy; run **smoke tests**.
5. **Production:** require **GitHub Environment** protection with required reviewers on `environment: production` (or equivalent). Repeat steps 3–4 for prod after approval.

## Rollback

- **Application:** revert Deployment image digest/tag to previous known-good via Git revert or automated rollback job; document in `payflow-infra-live/docs/rollback.md` and platform runbooks.
- **Terraform:** use stored state and `terraform apply` to previous module versions only with reviewed plan (never manual resource surgery as default narrative).

## Environment variable contract (informative)

| Source | Consumed by | Examples |
|--------|-------------|----------|
| Terraform outputs / Key Vault | `payflow-platform-config` (CSI / Secret) | DB URL, Service Bus connection string reference |
| Kubernetes Secret / CSI | `payflow-app` containers | `DATABASE_URL`, `PAYFLOW_QUEUE_BACKEND`, `REDIS_URL`, `AZURE_SERVICEBUS_CONNECTION_STRING` |
| CI OIDC | Terraform job | Azure subscription access without client secrets |

**Queue backend:** `PAYFLOW_QUEUE_BACKEND=redis` (default when unset) uses `REDIS_URL` for API/worker. `PAYFLOW_QUEUE_BACKEND=azservicebus` uses `AZURE_SERVICEBUS_CONNECTION_STRING` (namespace connection string from Terraform output `servicebus_primary_connection_string` or Key Vault). Queue names in Azure are `settlement`, `webhook`, `refund` (see `payflow-terraform-modules/modules/azure_servicebus`).

Exact names are defined alongside OpenAPI and manifests during implementation; this table is the **seam** between repos.

### Terraform module output contract (`examples/minimal`)

These logical names are what **`payflow-infra-live`** environments should mirror when composing the same modules (see `payflow-terraform-modules/examples/minimal/outputs.tf`):

| Logical output | Typical use |
|----------------|-------------|
| `resource_group_name` | Scope for Terraform data sources and RBAC assignments |
| `aks_subnet_id` | AKS `default_node_pool` / Azure CNI |
| `postgres_subnet_id` | PostgreSQL Flexible `delegated_subnet_id` |
| `virtual_network_id` | Private DNS links, future peering |
| `oidc_issuer_url` | Entra **federated credential** `issuer` for Kubernetes workload identity |
| `postgres_fqdn` | Database hostname segment in app connection strings (credentials from Key Vault) |
| `key_vault_uri` | Secrets Store CSI / application configuration |

## Definition of done for a release candidate

- [ ] Staging smoke: health + one authenticated payment flow + one webhook receipt path.
- [ ] No plaintext secrets in git diff.
- [ ] Dashboards show **non-zero** traffic or explicit “no data” panels for new metrics (documented).
- [ ] Runbook entry updated if new failure mode introduced.

## Documented solutions (informative)

When the **`payflow-app`** workflow fails on **Trivy** (missing action version or **exit code 1** on HIGH/CRITICAL image findings), the investigation and remediation are written up in:

- `docs/solutions/security-issues/payflow-ci-trivy-action-and-image-vulns.md`
