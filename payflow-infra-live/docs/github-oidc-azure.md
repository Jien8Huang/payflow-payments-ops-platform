# GitHub Actions OIDC → Azure (PayFlow infra)

**Purpose:** Run `terraform plan` / `apply` from GitHub without long-lived client secrets in repository variables (**R9**, **R18**).

## References

- [Authenticate to Azure from GitHub Actions using OpenID Connect](https://learn.microsoft.com/en-us/azure/developer/github/connect-from-azure-openid-connect)
- [Configuring OpenID Connect in Azure](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-azure)
- [Terraform azurerm: authenticating with OIDC](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/service_principal_oidc)

## Repository configuration

1. **Entra app registration** or **user-assigned managed identity** trusted for GitHub OIDC.
2. **Federated credential** subject matching this repository (and branch/environment if restricted), for example:
   - Issuer: `https://token.actions.githubusercontent.com`
   - Subject: `repo:OWNER/REPO:environment:dev` when using GitHub **environments**.
3. **GitHub repository secrets** (same names as `.github/workflows/terraform-plan.yml`):
   - `AZURE_CLIENT_ID`
   - `AZURE_TENANT_ID`
   - `AZURE_SUBSCRIPTION_ID`
4. **GitHub repository variable** `RUN_AZURE_PLAN_IN_CI` set to `true` when OIDC and remote backend are ready (otherwise only `fmt` + `validate` run on PRs).
5. **GitHub Environments** named `dev`, `staging`, and `production` (prod plan job) with **required reviewers** on `production` for separation-of-duties demos (**S3**).

## Workflow requirements

- Jobs that mint OIDC tokens must include:

```yaml
permissions:
  id-token: write
  contents: read
```

- Terraform must use OIDC-capable authentication (`ARM_USE_OIDC`, `ARM_CLIENT_ID`, etc.) as in the HashiCorp guide; `azure/login` sets these for subsequent steps.

## Backend state

Plan/apply jobs call `terraform init` **with** the remote backend. Commit a generated `backend.tf` from `backend.tf.example` **only** after the storage account exists, or inject `-backend-config` flags in CI — never commit storage account keys.
