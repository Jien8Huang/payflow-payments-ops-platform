# Module: `azure_aks`

## Purpose

Creates an **AKS** cluster with **Microsoft Entra Workload ID** prerequisites enabled (`workload_identity_enabled`, `oidc_issuer_enabled`), **Azure CNI** using the subnet from `azure_network`, and a **system-assigned** control plane identity.

## Outputs (data classification)

| Output | Classification |
|--------|----------------|
| `oidc_issuer_url` | Non-secret; needed for federated credentials. |
| `kube_config_raw` | **Secret** — treat as credential; avoid logging. Prefer OIDC + `az aks get-credentials` in pipelines. |

## Notes

- `dns_prefix` must be globally unique within Azure DNS for the region; derive `name_prefix` accordingly in live roots.
- Node pool uses `vnet_subnet_id` for **Azure CNI** node subnet from `azure_network`.
