# PayFlow authentication and RBAC

This document describes how **tenant isolation** and **principal types** work in `payflow-app` as of Unit 4.

## Principals

| Principal | How it is issued | Typical use |
|-----------|------------------|---------------|
| `api_key` | Created with the tenant (`POST /v1/tenants` returns the raw key once). Stored only as a SHA-256 hash. | Server-to-server merchant integrations. |
| `dashboard` | Issued by `POST /v1/auth/login` after email/password verification. JWT is HS256 with claims `tid` (tenant id), `sub` (dashboard user id), standard time claims. | Browser dashboard (future SPA). |

Every authenticated HTTP handler receives `tenant_id` and `principal` from middleware (`internal/auth/context.go`). Handlers that also accept a tenant id in the URL **must** compare it to the authenticated `tenant_id` and reject mismatches with **403** and a stable error code (`tenant_mismatch`).

## Middleware

- **`X-Api-Key`:** Resolved against `api_keys` where `revoked_at IS NULL`. Sets principal `api_key` and subject id = `api_keys.id`.
- **`Authorization: Bearer`:** Validated for dashboard JWTs. Sets principal `dashboard` and subject id = `dashboard_users.id`.
- **`/v1/tenants/me`:** Accepts **either** API key **or** Bearer JWT (`MeAuth`).
- **Mutations that require merchant context only** (e.g. `POST /v1/tenants/{tenantID}/dashboard-users`): Wrapped with **API key only** middleware so dashboard JWTs cannot bootstrap additional users without the integration key.

## Audit (R11)

Stable `event_type` strings include at minimum:

- `tenant_created`, `api_key_issued` — tenant onboarding.
- `dashboard_user_created` — first dashboard identity for a tenant.
- `dashboard_login_success`, `dashboard_login_failure` — authentication outcomes (failures may have `tenant_id` null when the JWT cannot be parsed).

## Operational notes

- JWT signing secret is injected via **`JWT_SECRET`** (see `internal/config`). Production values must never be committed.
- API keys are shown in plaintext **only** at creation; thereafter only hash and prefix are stored.
