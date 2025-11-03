# PayFlow — repository map to hiring signals

This matrix ties each repository to themes that recur in EU senior/staff DevOps, platform, and SRE postings (Azure/AKS, Terraform, CI/CD, security, observability, collaboration with product engineering). It implements **R26** in `docs/brainstorms/payflow-requirements.md`.

**Cross-repo contracts:** `docs/contracts/` (`idempotency.md`, `async-plane.md`, `release-checklist.md` with **Pattern A** CI layout).

| Hiring theme (condensed from role descriptions) | Primary repo | Secondary repo |
| --- | --- | --- |
| Design, build, maintain **cloud/platform** for reliable, secure, scalable production | `payflow-infra-live` | `payflow-terraform-modules` |
| **Terraform / IaC** for provisioning and controlled change | `payflow-terraform-modules` | `payflow-infra-live` |
| **CI/CD** (multi-stage, scans, **manual prod gates**) | `payflow-app` (image build/publish) | `payflow-infra-live` (plan/apply), `payflow-platform-config` (deploy triggers / GitOps) |
| **Kubernetes / AKS** lifecycle, ingress, workloads | `payflow-platform-config` | `payflow-infra-live` (cluster foundation) |
| **Reliability, performance, efficiency** (HPA, PDB, capacity, backoff) | `payflow-platform-config` | `payflow-app` (idempotency, queue consumers) |
| **Security**: secrets, least privilege, **OIDC** from CI to cloud, secure SDLC | `payflow-infra-live`, `payflow-terraform-modules` | `payflow-app` (dependency scan, container scan) |
| **Networking** (VNETs, LB, TLS, HTTP/S) | `payflow-infra-live` | `payflow-platform-config` (ingress, NetworkPolicy) |
| **Observability**: metrics, alerts, **SLIs/SLOs**, Grafana/Prometheus-class | `payflow-platform-config` | `payflow-app` (instrumentation) |
| **Collaboration with engineering**: contracts, release notes, deployment handoff | `payflow-app` | `payflow-platform-config` (image tags, rollout docs) |

**Default cloud:** Azure (AKS, VNET, Key Vault–class patterns). **Default CI reference:** GitHub Actions with environment protection on production; GitLab CI and Azure DevOps are documented as mapping notes where useful (`R18`).
