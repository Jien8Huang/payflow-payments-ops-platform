# Compliance considerations (PayFlow)

**Origin:** R24, N1. This document describes **engineering posture** aligned with common PCI DSS themes. It is **not** a PCI assessment, SAQ, or certification.

## Scope statement

PayFlow is a **portfolio / demonstration** system: **no real cardholder data**, **no PAN**, **no card network integrations**, and **no production money movement** per product requirements. Claims of “PCI compliance” or “certified” status are **out of scope** and must not appear in READMEs or hiring materials.

## Data the software is designed *not* to handle

- Primary account numbers (PAN), sensitive authentication data (SAD/CAV2/CVC), full track data, or PIN blocks.
- Raw magnetic stripe or chip payloads.

## Controls reflected in design (high level)

- **Secrets:** Production narrative assumes **Azure Key Vault** (or equivalent) and injection via CSI / workload identity — not plaintext in git (see `docs/contracts/release-checklist.md`).
- **Tenant isolation:** Application-layer enforcement and tests (`payflow-app`); optional PostgreSQL RLS is a documented future hardening path in the platform plan.
- **Auditability:** Structured audit events for security-relevant actions (see `docs/auth-rbac.md` and `payflow-app/internal/audit`).
- **Transport:** TLS at ingress (Kubernetes manifests) for customer-facing HTTP.

## Third parties and shared responsibility

Merchants, cloud providers, and payment networks each hold part of any real compliance boundary. This repo documents only the **application and platform** slice.

## When scope changes

If the product later stores payment credentials or moves real funds, this document must be superseded by a formal security/compliance program and legal review — not by incremental README edits alone.
