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

- **GitHub Actions:** `.github/workflows/payflow-app.yml` runs `gofmt`, `go vet`, `go mod verify`, and `go test ./... -count=1` on pushes and pull requests that touch `payflow-app/**`. Multi-stage **Docker** builds and **Trivy** image scans are documented in `docs/solutions/security-issues/payflow-ci-trivy-action-and-image-vulns.md` for teams extending CI.
- **Checksums:** `go.sum` is committed for reproducible module downloads.
- **Build:** from `payflow-app/`: `docker build -f Dockerfile --target api -t payflow-api:local .` and `--target worker` for the worker.

## Local run

- `docker compose up -d` starts Postgres and Redis.
- `go run ./cmd/api` starts the API on `LISTEN_ADDR` (default `:8080`). **Mock settlement** for new payments and refunds is recorded in the **`async_outbox`** table in the same database transaction as the business row; the worker drains that outbox and then enqueues webhooks on Redis when needed.
- **Queue backend:** `PAYFLOW_QUEUE_BACKEND` is unset or `redis` by default. Set **`REDIS_URL`** (e.g. `redis://127.0.0.1:6379/0`) so the API publishes and the worker consumes **`payflow:settlement_jobs`**, **`payflow:webhook_jobs`**, and **`payflow:refund_jobs`**. If **`REDIS_URL`** is unset on the API, the queue publisher is a no-op (webhook retry and DLQ republish return `503` when a real broker is required). For **Azure Service Bus** (staging/prod narrative), set **`PAYFLOW_QUEUE_BACKEND=azservicebus`** and **`AZURE_SERVICEBUS_CONNECTION_STRING`** (namespace connection string from Terraform output `servicebus_primary_connection_string` or Key Vault). SB queue names are **`settlement`**, **`webhook`**, **`refund`**.
- `go run ./cmd/worker` drains **`async_outbox`** (`payment_settlement`, `refund_settlement`) with row locking, then consumes settlement / webhook / refund jobs from the configured broker (Redis lists or Service Bus queues). Run the API (or `go run ./cmd/api`) once first so migrations apply to the database before starting the worker.
- **Refunds / webhooks (Unit 6):** `POST /v1/payments/{id}/refunds` (API key + `Idempotency-Key`), `PATCH /v1/tenants/me/webhook` to set URL + signing secret, `GET /v1/webhook-deliveries` and `.../{id}` (MeAuth), `POST /v1/webhook-deliveries/{id}/retry` for DLQ replay. Worker drains **`payflow:webhook_jobs`** and **`payflow:refund_jobs`** with timeouts and bounded retries; DLQ rows use `status=dlq`. Tune **`WEBHOOK_MAX_ATTEMPTS`** (default 5) via env on the worker process.
- **Metrics:** `GET /metrics` exposes Prometheus metrics including **`payflow_payments_created_total`** (incremented when a payment row is first created).
- **Dashboard (R7):** `payflow-dashboard/` — `npm run dev` proxies `/v1` to the API on `127.0.0.1:8080` (no CORS needed). If the SPA is hosted on another origin without a proxy, set **`CORS_ALLOWED_ORIGINS`** on the API (comma-separated `Origin` values, e.g. `http://localhost:5173`).
- **Tracing (optional):** when **`OTEL_EXPORTER_OTLP_ENDPOINT`** is set and **`OTEL_SDK_DISABLED`** is not `true`, the API and worker register an OTLP **HTTP** trace exporter. Use **`OTEL_SERVICE_NAME`** to override the default service name (`payflow-api` / `payflow-worker`). The endpoint value is passed to `otlptracehttp.WithEndpointURL` (include `/v1/traces` if your collector expects the full traces URL).
- Integration tests: `INTEGRATION=1 INTEGRATION_RESET=1 go test ./test/integration/...` (`INTEGRATION_RESET=1` drops application tables including `payments` and `ledger_events` — use only against disposable dev DBs).

## Does not belong here

Long-lived cloud credentials; Terraform beyond local developer convenience; full cluster definitions (those live in `payflow-infra-live` and `payflow-platform-config`).
