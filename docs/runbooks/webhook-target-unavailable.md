# Runbook: webhook target unavailable (R14 #4)

## Scenario

Merchant HTTPS endpoint returns **5xx**, times out, or resets connections during outbound `webhook_deliveries` processing.

## Expected system behavior

Bounded retries with backoff; after `max_attempts`, row moves to **`dlq`** status with `last_error` populated. Merchant must treat deliveries as **at-least-once** (`docs/contracts/async-plane.md`).

## Signals

- Table `webhook_deliveries` where `status = 'dlq'` or `attempt_count` rising.
- HTTP client errors in worker logs (`http_5xx`, dial errors).
- Merchant reports duplicate events → verify **receiver idempotency** using `merchant_idempotency_key`.

## Mitigations

- Use **GET `/v1/webhook-deliveries`** and detail endpoint to inspect DLQ; re-drive manually after merchant fixes endpoint (operational procedure outside this repo).
- Tune `WEBHOOK_MAX_ATTEMPTS` only with product agreement (trade-off vs DLQ noise).
- Signing secret rotation during in-flight retries: see `docs/contracts/async-plane.md` (hard cutover vs dual-secret verification).

## Dashboards / links

- Stub Prometheus rules: `payflow-platform-config/observability/prometheus/configmap-rules.yaml` — replace with real alert on DLQ rate.
- OpenAPI: webhook delivery list/detail routes in `payflow-app/api/openapi.yaml`.
