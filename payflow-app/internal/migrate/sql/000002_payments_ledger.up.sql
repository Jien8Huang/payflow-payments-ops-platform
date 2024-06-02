-- Payments (R4): idempotency scoped per tenant + scope + key.
CREATE TABLE payments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    amount_cents        BIGINT NOT NULL,
    currency            CHAR(3) NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending',
    idempotency_scope   TEXT NOT NULL,
    idempotency_key     TEXT NOT NULL,
    request_hash        TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payments_amount_positive CHECK (amount_cents > 0),
    CONSTRAINT payments_idempotency_unique UNIQUE (tenant_id, idempotency_scope, idempotency_key)
);

CREATE INDEX payments_tenant_id_idx ON payments (tenant_id);
CREATE INDEX payments_tenant_status_idx ON payments (tenant_id, status);

-- Append-only ledger (R5); consumer idempotency via (payment_id, dedupe_key).
CREATE TABLE ledger_events (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    payment_id  UUID NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    dedupe_key  TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (payment_id, dedupe_key)
);

CREATE INDEX ledger_events_tenant_id_idx ON ledger_events (tenant_id);
CREATE INDEX ledger_events_payment_id_idx ON ledger_events (payment_id);
