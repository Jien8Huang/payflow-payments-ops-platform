# payflow-platform-config

## Purpose

**Runtime platform** configuration for **AKS**: Kubernetes manifests (**Kustomize** by default), **ingress + TLS**, **HorizontalPodAutoscaler**, **PodDisruptionBudget**, **NetworkPolicy**, namespace-scoped **RBAC**, and **observability as code** (Prometheus rules, Grafana dashboards, alert routes — **Unit 8**). **GitOps** (Flux / Argo CD) is **optional phase 2** per plan — phase 1 may use pipeline-driven `kubectl` / `kustomize` deploys.

## Layout

| Path | Role |
| --- | --- |
| `base/` | Namespace `payflow`, ServiceAccounts (Entra **workload identity** annotations), Deployments (`payflow-api`, `payflow-worker`), Service, Ingress (`ingressClassName: nginx`), PDB, HPA. Image names `payflow-api` / `payflow-worker` are rewritten in overlays. |
| `overlays/dev` | Single-replica defaults, small CPU/memory, HTTP-only Ingress host `api.dev.payflow.local`, HPA max 3. |
| `overlays/staging` | 2 replicas, TLS + `cert-manager.io/cluster-issuer: letsencrypt-staging`, higher limits than dev, HPA max 15, PDB `minAvailable: 1`. |
| `overlays/prod` | 3 replicas, TLS + `letsencrypt-prod`, largest requests/limits, HPA max 30, PDB `minAvailable: 2`. |
| `policies/network/` | Default-deny for `app.kubernetes.io/part-of=payflow`, allow Ingress from `ingress-nginx`, allow egress DNS + 443/5432/6379 (tighten when private endpoints are fixed). |
| `policies/rbac/` | Namespace **Role** + **RoleBinding** for read-only troubleshooting (`Group: payflow-namespace-readers` — replace with Entra group OIDC binding in your cluster). |

## Interfaces to sibling repos

| Peer | Interface |
| --- | --- |
| `payflow-app` | Container images (`ghcr.io/payflow/payflow-api` and `…/payflow-worker` placeholders); binaries must listen on **:8080** for API probes; worker uses `DATABASE_URL`, `REDIS_URL` from env. |
| `payflow-infra-live` | AKS + **OIDC issuer**; Terraform outputs feed **managed identity client ids** patched into `ServiceAccount` annotations (`azure.workload.identity/client-id`). Federated credential subject must match the Kubernetes service account (namespace + name). |
| `.github/workflows/` (repo root) | **Pattern A:** manifest validation uses `paths: payflow-platform-config/**` (Unit 8). |

## Hiring signals addressed

AKS **operations**, **SRE** observability (metrics, logs, traces wiring), **SLI/SLO** artifacts, **capacity** and **disruption** controls (PDB/HPA), collaboration via **documented image promotion** across overlays.

## Observability (as code)

- `observability/` — Kustomize bundle (namespace `payflow`): Prometheus rule **stub** ConfigMap and Grafana dashboard **stub** ConfigMap (`grafana_dashboard: "1"` label). Wire scrape configs and operator-sidecar patterns in your cluster; see `docs/slo/payment-api-slo.md` for SLI placeholders.

## Does not belong here

Terraform that provisions the **cluster foundation** (that remains in `payflow-terraform-modules` + `payflow-infra-live` unless planning documents a deliberate exception).

## Bootstrap (before first apply)

1. Create namespace-scoped secret **`payflow-runtime`** (not committed to git). Keys consumed by manifests today:

   - `DATABASE_URL` — Postgres URL for API and worker.
   - `JWT_SECRET` — API only.
   - `REDIS_URL` — API and worker (broker).

   Example (local / break-glass only; use Key Vault + CSI or External Secrets in production):

   ```bash
   kubectl -n payflow create secret generic payflow-runtime \
     --from-literal=DATABASE_URL='postgres://...' \
     --from-literal=JWT_SECRET='...' \
     --from-literal=REDIS_URL='redis://...'
   ```

2. Replace **placeholder** Ingress hosts, cert-manager **cluster issuers**, and **image tags/digests** in overlays to match your registry and DNS.

3. Set **real** `azure.workload.identity/client-id` values on both ServiceAccounts (per overlay patch files or Terraform-driven `kustomize` replacements).

## Verification

From the repository root:

```bash
scripts/ci/validate-platform-config.sh
```

This runs `kubectl kustomize` (or `kustomize build`) on `overlays/dev`, `overlays/staging`, and `overlays/prod`.

## HPA vs queue depth

Scaling the **worker** on Redis queue depth is **deferred** (KEDA or custom metrics adapter). The worker uses fixed replica counts per overlay; only the **API** has an HPA on CPU in this unit.
