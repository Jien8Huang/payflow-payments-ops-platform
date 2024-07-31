# SQL migrations

Executable migration files live under **`internal/migrate/sql/`** so they can be embedded with `//go:embed` (Go forbids `..` in embed paths). This directory documents the same ordering and naming for DBAs reviewing diffs in PRs.

Ordering includes **`000002_payments_ledger`** (payments + append-only `ledger_events`) after the initial tenant/auth schema, then **`000003_refunds_webhooks`** (tenant webhook columns, `refunds`, `webhook_deliveries`).
