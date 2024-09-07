# Module: `azure_network`

## Purpose

Creates a **resource group**, **virtual network**, a **non-delegated subnet for AKS**, and a **delegated subnet for PostgreSQL Flexible Server** (delegation `Microsoft.DBforPostgreSQL/flexibleServers`).

## Network policy engines (AKS)

AKS **NetworkPolicy** enforcement depends on the CNI / dataplane (Azure NPM, Cilium, etc.). This module **does not** enable a policy add-on; `payflow-platform-config` and cluster creation (`azure_aks`) choose the policy story. See Microsoft Learn: [Secure pod traffic with network policies in AKS](https://learn.microsoft.com/en-us/azure/aks/use-network-policies) and [Best practices for network policies in AKS](https://learn.microsoft.com/en-us/azure/aks/network-policy-best-practices).

## Inputs / outputs

See `variables.tf` and `outputs.tf`. **Data classification:** all outputs are **non-secret** resource identifiers; no credentials.

## Constraints

- `postgres_subnet_cidr` must not overlap other subnets and must stay inside `address_space`.
- PostgreSQL Flexible Server requires a **dedicated** delegated subnet; do not attach unrelated NICs to that subnet per Microsoft guidance.
