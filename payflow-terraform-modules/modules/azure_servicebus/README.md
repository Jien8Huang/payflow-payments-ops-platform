# Azure Service Bus module (PayFlow)

Creates a **namespace** and three **queues** aligned with `payflow-app` / `docs/contracts/async-plane.md`:

- `settlement` — payment settlement jobs
- `webhook` — outbound webhook delivery jobs
- `refund` — refund settlement jobs

## Outputs

- `primary_connection_string` (sensitive): use for local CI or bootstrap only; in AKS prefer **workload identity** + RBAC and omit root connection strings from app pods where possible.

## Inputs

See `variables.tf`. Default SKU is **Standard** (queues + DLQ behavior used by the app narrative).
