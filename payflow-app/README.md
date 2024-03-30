# payflow-app

## Purpose

Merchant-facing **HTTP APIs** and **workers** (authentication, payments, refunds, ledger-style events, webhook delivery, background jobs). This tree is where **application correctness** is demonstrated: **multi-tenant isolation**, **idempotency**, simulated payment state, **structured logs** (`request_id`, `tenant_id`, `service_name`, `event_type`, optional `trace_id`), and hooks for metrics and traces.

## Interfaces to sibling repos

| Sibling | Interface |
| --- | --- |
| `payflow-infra-live` / Terraform outputs | Injected **secrets** and **URLs** (database, broker) via Kubernetes; see `docs/contracts/release-checklist.md` env contract table. |
| `payflow-platform-config` | **Container image** digest/tag, **resource requests/limits**, **probes**, **ServiceAccount** for workload identity. |
| `docs/contracts/` | **Idempotency**, **async plane**, and **release order** are normative for API and worker behavior. |

## Security and compliance notes

- **Dual auth:** dashboard users (JWT/session path) vs **API keys** for integrations — see requirements **R2** and repo-root `docs/auth-rbac.md`.
- **Tenant isolation:** every mutating and sensitive read path resolves **tenant_id** from the authenticated principal — requirements **R8**.
- **Audit:** minimum events listed in **R11**; audit writes are side effects with stable `event_type` strings.
- **No PAN/CVV** — mock or tokenized payloads only (**R12**).

## Hiring signals addressed

CI/CD for **build, test, security scan, container publish**; handoff via **immutable image digests**; **secure SDLC** (dependency and image scanning); **Go** services plus **scripting** (Make/Python) per plan.

## CI and containers

- **GitHub Actions:** `.github/workflows/payflow-app.yml` runs `gofmt`, `go vet`, `go mod verify`, `go test ./...`, multi-stage **Docker** builds (`--target api` / `--target worker`), and a **Trivy** scan on the API image.
- **Checksums:** `go.sum` is committed for reproducible module downloads.
- **Build:** from `payflow-app/`: `docker build -f Dockerfile --target api -t payflow-api:local .` and `--target worker` for the worker.

## Local run

- `docker compose up -d` starts Postgres and Redis.
- `go run ./cmd/api` starts the API on `LISTEN_ADDR` (default `:8080`). Set **`REDIS_URL`** (e.g. `redis://127.0.0.1:6379/0`) so accepted payments enqueue mock settlement work; if unset, the API uses a no-op queue (payments stay `pending` until you run settlement manually or wire Redis).
- `go run ./cmd/worker` consumes **`payflow:settlement_jobs`** from Redis and advances `pending` → `succeeded` with idempotent ledger writes. Run the API (or `go run ./cmd/api`) once first so migrations apply to the database before starting the worker.
- **Refunds / webhooks (Unit 6):** `POST /v1/payments/{id}/refunds` (API key + `Idempotency-Key`), `PATCH /v1/tenants/me/webhook` to set URL + signing secret, `GET /v1/webhook-deliveries` and `.../{id}` (MeAuth). Worker drains **`payflow:webhook_jobs`** and **`payflow:refund_jobs`** with timeouts and bounded retries; DLQ rows use `status=dlq`. Tune **`WEBHOOK_MAX_ATTEMPTS`** (default 5) via env on the worker process.
- **Metrics (Unit 8):** increment a `payments_created` counter (or equivalent) when `payment.Service.Create` returns `created=true` — hook point is documented in `internal/payment/payment.go`.
- Integration tests: `INTEGRATION=1 INTEGRATION_RESET=1 go test ./test/integration/...` (`INTEGRATION_RESET=1` drops application tables including `payments` and `ledger_events` — use only against disposable dev DBs).

## Does not belong here

Long-lived cloud credentials; Terraform beyond local developer convenience; full cluster definitions (those live in `payflow-infra-live` and `payflow-platform-config`).
