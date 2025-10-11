# Runbook: cluster maintenance / node drain (R14 #6)

## Scenario

AKS upgrades, node image updates, or manual `kubectl drain` evicts pods.

## Expected system behavior

**PodDisruptionBudget** on `payflow-api` preserves minimum availability (`payflow-platform-config` overlays: prod uses higher `minAvailable` than staging). Worker is intentionally fixed-replica in v1; scale HPA for API, not worker queue depth (see `payflow-platform-config/README.md`).

## Signals

- `kubectl get pdb -n payflow`.
- Cluster autoscaler / upgrade notifications from Azure.
- API availability dip correlated with drain windows.

## Mitigations

- Schedule maintenance outside merchant peak if known; temporarily raise replicas in overlay after review.
- If PDB blocks drain, **do not** delete PDB without replacement controls — widen maintenance window or add temporary capacity.
- Validate **NetworkPolicy** still allows ingress from `ingress-nginx` after CNI or ingress upgrades.

## Dashboards / links

- `payflow-platform-config/base/pdb-api.yaml` and prod/staging patches.
- `payflow-platform-config/policies/network/` — ingress/egress policies.
