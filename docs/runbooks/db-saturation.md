# Runbook: database saturation / overload (R14 #3)

## Scenario

Postgres connection pool exhaustion, elevated query latency, or storage IOPS limits cause API **503** / timeouts.

## Expected system behavior

API returns **503** with opaque `request_id` where designed; clients back off. No partial double-charge: transactional boundaries around payment + outbox remain a documented design goal (`docs/contracts/async-plane.md`).

## Signals

- `pg_stat_activity` count near `max_connections`; active wait events (`pg_stat_activity.wait_event_type`).
- API latency histogram tail; pool **acquire timeout** logs from `pgx`.
- AKS node disk queue depth on database tier (Azure monitor).

## Mitigations

- Increase pool size **carefully** (avoid thundering herd); prefer **horizontal API scaling** within DB limits.
- Add read replica only for read paths if product grows (not in v1 scope).
- **v1 load test:** this portfolio does **not** ship a k6 job in CI; capacity validation is **runbook + manual** unless you add a dedicated perf pipeline.

## Dashboards / links

- Azure Flexible Server metrics (CPU, storage, connections).
- `docs/slo/payment-api-slo.md` — latency/error SLIs once wired.
