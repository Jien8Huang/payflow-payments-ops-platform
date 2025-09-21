# Idempotency contract (PayFlow)

**Origin:** `docs/brainstorms/payflow-requirements.md` (R4).  
**Plan:** `docs/plans/payflow-platform-plan.md`.

## Scope

- **Payment submission:** HTTP mutating create must accept an **Idempotency-Key** (or equivalent header name fixed in OpenAPI). Retries with the same key return the **same logical outcome** as the first completed interpretation for that key.
- **Refund submission:** Same rule applies when the API exposes a refund create that accepts an idempotency key. If refunds are keyed only by payment reference, that alternative is documented in OpenAPI and must still be **safe under retry** (same refund intent → one refund record).

## Uniqueness

- Keys are scoped per **tenant** and per **operation scope** (for example `payment:create` vs `refund:create`). The pair `(tenant_id, idempotency_scope, idempotency_key)` is **unique** in persistent storage.
- Two different tenants may send the same key string; they must **not** collide.

## Concurrency

- Concurrent two requests with the same key and **identical body:** at most one payment (or refund) is created; others receive the **same** resource identity and equivalent response.
- Same key with **different body:** respond with **409 Conflict** (or documented Stripe-like mismatch behavior) and do **not** partially apply the second body.

## Retention

- Keys are retained long enough to cover client retry windows (target: **24 hours** minimum; align with queue visibility where applicable). Document actual TTL in implementation notes when storage uses expiration.

## Queue consumers

- Message delivery is **at-least-once**. Financial correctness relies on **consumer idempotency** (dedupe by business key, unique constraints, or processed-event ledger). See `docs/contracts/async-plane.md`.
