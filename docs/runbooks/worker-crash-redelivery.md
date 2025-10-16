# Runbook: worker crash mid-job (R14 #2)

## Scenario

`payflow-worker` dies after dequeuing a job (settlement, webhook delivery, refund settlement) but before finishing side effects.

## Expected system behavior

Queue drivers are **at-least-once**; handlers must be idempotent (`payment.SettleMock`, `webhook.ProcessDelivery`, `refund.SettleMock` use deterministic ledger keys / status checks).

## Signals

- Redis / Service Bus **visibility timeout** / retry metrics (when broker metrics exist).
- Worker pod **restart count** and **OOMKilled** events (`kubectl describe pod`).
- Logs: same `payment_id` / `delivery_id` processed twice without duplicate ledger rows.

## Mitigations

- Scale worker replicas; verify **PDB** allows enough capacity during drains (`docs/runbooks/node-drain-maintenance.md`).
- If duplicate ledger transitions appear, stop worker, snapshot DB, open incident, review idempotency keys in code paths.
- For webhook storms, see `docs/runbooks/webhook-target-unavailable.md`.

## Dashboards / links

- Kubernetes: deployment `payflow-worker` CPU/memory in Grafana (when wired).
- `docs/contracts/async-plane.md` — broker semantics.
