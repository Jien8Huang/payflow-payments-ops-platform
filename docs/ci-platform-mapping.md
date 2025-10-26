# CI/CD platform mapping (PayFlow)

**Purpose:** This repository uses **GitHub Actions** at the repo root (**Pattern A** in `docs/contracts/release-checklist.md`). This file maps jobs to equivalent concepts elsewhere.

| GitHub Actions | GitLab CI | Azure DevOps |
| --- | --- | --- |
| `workflow` + `job` | `stages` / `include` | `pipeline` / `stage` |
| `actions/setup-go` | `image: golang` + cache | `GoTool` task or Microsoft-hosted `ubuntu` + `UseGoVersion` |
| `actions/checkout` | `git clone` in job | `Checkout` step |
| `environment: production` + required reviewers | protected environments / manual gates | environment approvals |
| OIDC `azure/login` | OIDC JWT with Azure | workload identity federation service connection |
| `dependabot.yml` | Renovate / Dependency scanning | Dependabot equivalent: **ADO dependency scanning** or third-party |
| `gitleaks` action | **gitleaks** job in `.gitlab-ci.yml` | **Credential Scanner** or gitleaks in pipeline |
| `aquasecurity/trivy-action` | **Trivy** container job | Microsoft Defender for Cloud container scans / Trivy task |

Terraform plan/apply for Azure remains in `.github/workflows/terraform-plan.yml` with optional `RUN_AZURE_PLAN_IN_CI` (see `payflow-infra-live/docs/github-oidc-azure.md` if present).
