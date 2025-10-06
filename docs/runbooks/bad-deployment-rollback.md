# Runbook: bad deployment — detection and rollback (R14 #5)

## Scenario

A new `payflow-api` or `payflow-worker` image introduces regressions (5xx spike, failed migrations, crash loops).

## Expected system behavior

Kubernetes keeps previous ReplicaSet; rollback is **image digest / tag revert** via Git or pipeline. Database migrations follow **additive-first** ordering in `docs/contracts/release-checklist.md`.

## Signals

- Deployment **progressing=false**, surge of **CrashLoopBackOff**.
- Error budget burn in `docs/slo/payment-api-slo.md` metrics once wired.
- Synthetic `/healthz` failures from outside the cluster.

## Mitigations

- **Immediate:** `kubectl rollout undo deployment/payflow-api -n payflow` (or re-apply previous overlay image tag / digest).
- **Database:** if a migration already applied, **do not** `rollout undo` alone without compatibility analysis — follow `payflow-infra-live/docs/rollback.md` if present.
- Post-incident: add integration test or canary gate before replaying the change.

## Dashboards / links

- `payflow-platform-config/overlays/prod` — image tags / resource limits differ from staging by design.
- `docs/contracts/release-checklist.md` — promotion order.
