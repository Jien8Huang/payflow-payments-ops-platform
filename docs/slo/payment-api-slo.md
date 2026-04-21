# PayFlow payment API — SLI / SLO (draft)

**Scope:** HTTP API (`payflow-api`) serving merchant and dashboard traffic. **Origin:** R20. **Plan:** Unit 8.

## SLIs (to wire when metrics exist)

| SLI | Definition | Current signal |
| --- | --- | --- |
| Availability | Ratio of successful `GET /healthz` or synthetic probes vs attempts | **Not wired** — add Prometheus scrape + `up` or blackbox exporter. |
| Latency | p95 / p99 request duration for `POST /v1/payments`, `GET /v1/payments/{id}` | **Not wired** — add histogram from Go middleware or ingress metrics. |
| Error rate | HTTP 5xx / (2xx+3xx+4xx+5xx) excluding client 4xx if desired | **Not wired** — ingress `nginx_*` or app counter `http_requests_total`. |

## SLO targets (placeholders)

Document targets after one baseline window in staging (example placeholders only):

- **Availability:** 99.9% monthly (synthetic or `up{job="payflow-api"}`).
- **Latency:** p99 under 500ms for read paths under nominal load (tune per region).
- **Errors:** 5xx rate under 0.1% excluding known dependency maintenance windows.

## Error budget policy

When burn rate exceeds policy, freeze non-critical releases and focus on reliability work; record decisions in `docs/runbooks/bad-deployment-rollback.md` and the change ticket.

## Distributed tracing (R23)

**Current state:** `payflow-app` includes an **optional** OTLP HTTP exporter (`internal/tracing`) that is **disabled by default** and only activates when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (and `OTEL_SDK_DISABLED` is not `"true"`). This document treats tracing as **not wired** until platform scrape/export plumbing and trace context propagation are demonstrated end-to-end.

When wiring is completed, document trace_id propagation from ingress → API → worker and sampling rate here.

## Dashboards

Stub dashboard ConfigMap: `payflow-platform-config/observability/grafana/configmap-dashboard.yaml`. Replace JSON with panels bound to the SLI queries above once metrics exist.
