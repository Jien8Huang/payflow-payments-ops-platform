# Runbook: duplicate submission / idempotent client retry (R14 #1)

## Scenario

Clients retry `POST /v1/payments` or `POST /v1/payments/{id}/refunds` with the same `Idempotency-Key` after timeouts, double-clicks, or mobile flaky networks.

## Expected system behavior

First request wins; replays with the **same body** return **200** with the same resource id; replays with a **different body** return **409** (`idempotency_conflict`). Ledger and financial transitions must not duplicate (Unit 5–6 tests).

## Signals

- HTTP **409** rate on payment/refund create paths (ingress or app metric when wired).
- Spike in **duplicate** `request_id` logs without matching new payment rows.
- Database: count of rows grouped by `(tenant_id, idempotency_scope, idempotency_key)` should stay **1** per key.

## Mitigations

- Confirm clients send stable `Idempotency-Key` per logical operation (Stripe-style).
- Inspect `ledger_events` for duplicate settlement transitions; if found, treat as **sev-1** data integrity incident.
- Roll back bad client release that mutates body between retries.

## Dashboards / links

- Stub: `payflow-platform-config/observability/grafana/configmap-dashboard.yaml` — add panels on `http_requests_total{code="409"}` when metrics exist.
- SLO context: `docs/slo/payment-api-slo.md`.
