# Async plane contract (PayFlow)

**Origin:** R13, R6, R14. **Plan:** `docs/plans/payflow-platform-plan.md`.

## Brokers

- **Staging / production:** Azure Service Bus **queues** (or topics + subscriptions if the design uses pub/sub). Terraform creates namespace-scoped queues named **`settlement`**, **`webhook`**, and **`refund`** (`payflow-terraform-modules/modules/azure_servicebus`). Authentication via **connection string** (bootstrap / CI) or **workload identity** / managed identity per AKS docs for production hardening.
- **Local development:** Redis (Streams or list + consumer pattern) behind the same **Go internal interface** as Service Bus. Semantic differences (ordering, duplicate detection, DLQ) are documented here and in code comments on the interface. Redis list names remain **`payflow:settlement_jobs`**, **`payflow:webhook_jobs`**, **`payflow:refund_jobs`**.

## Outbox / enqueue

- Payment accepted in the API must not enqueue work in a second transaction that can commit if the first rolls back. Prefer **transactional outbox** (same DB transaction writes payment row + outbox row) or a single atomic write pattern documented in the implementation.

## Worker semantics

- Handlers must be **idempotent** under redelivery: duplicate messages must not create duplicate ledger transitions for the same logical step. Use **deterministic event identifiers** or unique constraints per `(payment_id, transition_name)` (exact schema deferred to implementation).

## Webhooks

- Deliveries are **signed**; merchants should verify signatures.
- Retries use **bounded exponential backoff**; after threshold, record **dead-letter** state.
- **Merchant receivers** should treat deliveries as **at-least-once** and dedupe by `event_id` (or documented field). The platform documents this responsibility; it does not dedupe on behalf of the merchant beyond its own delivery ledger.

## Signing secret rotation

- During rotation, **multiple signing secrets** may be valid for a bounded window if implementation chooses dual-secret verification; otherwise document **hard cutover** and expected transient webhook failures counted toward DLQ.

## PayFlow v1 HTTP webhook shape (`payflow-app`)

- **Headers:** `X-Payflow-Timestamp` (Unix seconds as decimal string), `X-Payflow-Signature` (lowercase hex HMAC-SHA256 of `timestamp + "." + raw_body` using the tenant’s configured signing secret).
- **Body:** JSON `payload` stored on the `webhook_deliveries` row (same bytes are POSTed to the merchant URL).
- **Hard cutover during rotation:** in-flight retries may still be signed with the previous secret until the worker finishes its attempt window; merchants that only accept the new secret will return **4xx/5xx**, which increments `attempt_count` and can move the row to **`dlq`** after `max_attempts`. Plan for dual-secret verification on the merchant side if zero-downtime rotation is required.

## Merchant receiver idempotency (reminder)

- Treat webhook POSTs as **at-least-once**. Dedupe using `merchant_idempotency_key` from the JSON payload contract (or the documented header if you add one later); the platform’s `webhook_deliveries` table dedupes **enqueue** per tenant + merchant key, not the merchant’s downstream side effects.
