# PayFlow

This directory holds **PayFlow** portfolio material: requirements under `docs/brainstorms/`, **cross-repo contracts** under `docs/contracts/`, the **implementation plan** under `docs/plans/`, and **four scaffold folders** that correspond to **four future git repositories** (`R25` in the requirements doc). Separate remotes are not created here; split when you are ready to version and permission each lifecycle independently.

| Folder | Role |
| --- | --- |
| `payflow-app` | Application services, tests, container build CI |
| `payflow-terraform-modules` | Reusable Terraform modules |
| `payflow-infra-live` | Per-environment Terraform roots and governance |
| `payflow-platform-config` | Kubernetes manifests, observability and alerts as code |

### Repository diagram

```mermaid
flowchart TB
  ROOT[PayFlow portfolio repo]
  ROOT --> docFldr["docs/: brainstorms, contracts, plans, runbooks, solutions/"]
  ROOT --> payApp["payflow-app/: Go API, worker, tests, Dockerfile"]
  ROOT --> tfmod["payflow-terraform-modules/: reusable Terraform"]
  ROOT --> infralive["payflow-infra-live/: per-environment roots"]
  ROOT --> pconf["payflow-platform-config/: Kustomize + observability"]
  ROOT --> gh[".github/: Actions, Dependabot"]
```

## Test-driven development (TDD)

For **`payflow-app`**, automated tests are the executable counterpart to the written contracts under `docs/contracts/` and the requirements under `docs/brainstorms/`. The intended loop is: **read the contract or requirement → write a failing test (red) → implement the smallest change that passes (green) → refactor while keeping tests and `gofmt`/`go vet` green**.

### TDD loop and where tests run

```mermaid
flowchart TB
  subgraph loop [Documentation-driven loop]
    DOC["docs/contracts + docs/brainstorms"]
    RED["Red: add or change *_test.go"]
    APP["payflow-app: cmd/ + internal/"]
    GRN["Green: go test passes"]
    REF["Refactor + gofmt + go vet + go mod verify"]
    DOC --> RED --> APP --> GRN --> REF
    REF --> DOC
  end

  subgraph layers [Same repo: two test layers]
    U["Unit: internal/*/*_test.go — always executed"]
    I["Integration: test/integration — needs INTEGRATION=1 + Postgres"]
  end
  RED --> U
  RED --> I
  APP --> U
  APP --> I

  subgraph ci [Default GitHub Actions payflow-app job]
    JOB["go test ./... -count=1"]
  end
  U --> JOB
  I --> JOB
```

In the **CI** subgraph, the `test/integration` package is still **loaded** by `go test`, but individual tests call **`t.Skip`** when `INTEGRATION` is unset, so only **unit** assertions run in the default workflow.

### Where tests live

| Layer | Location | Role |
| --- | --- | --- |
| **Unit** | `payflow-app/internal/*/*_test.go` (e.g. `internal/auth`, `internal/payment`, `internal/tenant`) | Fast checks on pure logic: hashing, JWT round-trips, idempotency fingerprints, parsers. Uses `testing` and `t.Parallel()` where tests are independent. No database or network. |
| **Integration** | `payflow-app/test/integration/*_test.go` | End-to-end HTTP behavior against a real **PostgreSQL** URL, schema from **`internal/migrate`**, and an in-process server via **`httptest`**. Uses shared helpers in `helpers_test.go` (router wiring, tenant creation, optional table reset). |

Representative integration themes covered in code today: **payment idempotency under concurrency**, **tenant isolation**, **refund state**, **webhook delivery and DLQ-style API paths**, **onboarding and API keys**.

### Running tests locally

From the repo root:

```bash
cd payflow-app
go test ./... -count=1
```

That command runs all packages: unit tests always execute; **integration tests skip unless the environment gate is set** (each file checks `INTEGRATION`).

To run integration tests against a database you control:

```bash
cd payflow-app
export INTEGRATION=1
export DATABASE_URL='postgres://USER:PASSWORD@HOST:5432/DBNAME?sslmode=disable'
export JWT_SECRET='integration-test-jwt-secret'   # matches helpers used in tests
go test ./... -count=1 -v
```

Optional: `export INTEGRATION_RESET=1` so helpers **drop and recreate** PayFlow tables before migrations (destructive for that database; use a disposable database or schema).

`config.Load()` reads `DATABASE_URL`, `JWT_SECRET`, and related settings from the environment. If `DATABASE_URL` is empty, it falls back to `postgres://payflow:payflow@localhost:5432/payflow?sslmode=disable`; if `JWT_SECRET` is empty, it falls back to a dev placeholder (see `payflow-app/internal/config/config.go`).

### How this maps to CI

The **`payflow-app`** workflow runs `go test ./... -count=1` **without** `INTEGRATION=1`, so **CI always runs unit tests** and **integration tests report as skipped** unless you extend the workflow with a Postgres service and set `INTEGRATION` (not configured in the default workflow today).

### Conventions useful for TDD in this repo

- Prefer **`t.Helper()`** in shared setup (already used in integration helpers) so failures point at the calling test line.
- Keep **new behavior** behind a test that fails first; for HTTP features, add or extend a file under `test/integration/` and reuse `testHTTPServer` / `mustCreateTenant` where possible.
- After edits, match CI locally: **`gofmt`**, **`go vet ./...`**, **`go mod verify`**, **`go test ./... -count=1`** (same sequence as `.github/workflows/payflow-app.yml`).

## Authoritative documents

| Artifact | Path |
| --- | --- |
| Requirements | `docs/brainstorms/payflow-requirements.md` |
| Technical plan | `docs/plans/payflow-platform-plan.md` |
| EU hiring signal map | `docs/portfolio-signals.md` |
| Idempotency contract | `docs/contracts/idempotency.md` |
| Async / queue / webhook contract | `docs/contracts/async-plane.md` |
| Release checklist + **CI layout (Pattern A)** | `docs/contracts/release-checklist.md` |
| SLO draft (payment API) | `docs/slo/payment-api-slo.md` |
| Compliance posture (no certification claims) | `docs/compliance-considerations.md` |
| Runbooks (six R14 scenarios) | `docs/runbooks/` |
| CI platform mapping (GitHub → GitLab / ADO) | `docs/ci-platform-mapping.md` |
| Documented solutions (bugs, security, practices; YAML frontmatter) | `docs/solutions/` |

## CI layout (summary)

**Pattern A (locked):** all GitHub Actions workflows live at **`.github/workflows/`** in this repo root with `paths` filters per logical repo. Details: `docs/contracts/release-checklist.md`.

**Infra CI:** `.github/workflows/terraform-plan.yml` — `terraform fmt`, `terraform validate` (all envs); optional Azure OIDC `plan` when repo variable `RUN_AZURE_PLAN_IN_CI` is set (see `payflow-infra-live/docs/github-oidc-azure.md`).

**App CI:** `.github/workflows/payflow-app.yml` — Go fmt/vet/verify/test, Docker build (`api` + `worker` targets), Trivy scan.

**Platform CI:** `.github/workflows/platform-config.yml` — `kubectl kustomize` for overlays + `observability/`, YAML parse via `scripts/ci/lint-yaml.sh`.

**Secret scanning:** `.github/workflows/gitleaks.yml` on pushes and pull requests.

**Dependabot:** `.github/dependabot.yml` for `payflow-app` Go modules and GitHub Actions.

## Operational boundaries

- **No** production payment processing, **no** real card data, **no** PCI certification claims (see requirements **N1**, **R24**).
- Secrets and cloud authentication follow **OIDC** and **Key Vault** patterns described in the plan; nothing long-lived belongs in git.
