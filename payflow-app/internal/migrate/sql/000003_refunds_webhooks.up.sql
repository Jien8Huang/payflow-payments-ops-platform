-- Tenant webhook target (R9: production secret from Key Vault; local column for dev).
ALTER TABLE tenants
    ADD COLUMN webhook_url TEXT,
    ADD COLUMN webhook_signing_secret TEXT;

CREATE TABLE refunds (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    payment_id          UUID NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    amount_cents        BIGINT NOT NULL,
    currency            CHAR(3) NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending',
    idempotency_scope   TEXT NOT NULL,
    idempotency_key     TEXT NOT NULL,
    request_hash        TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT refunds_amount_positive CHECK (amount_cents > 0),
    CONSTRAINT refunds_idem UNIQUE (tenant_id, idempotency_scope, idempotency_key)
);

CREATE INDEX refunds_tenant_id_idx ON refunds (tenant_id);
CREATE INDEX refunds_payment_id_idx ON refunds (payment_id);

-- Outbound merchant webhooks (R6): DLQ is status = dlq; attempts bounded by max_attempts.
CREATE TABLE webhook_deliveries (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                  UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    payment_id                 UUID REFERENCES payments (id) ON DELETE SET NULL,
    refund_id                  UUID REFERENCES refunds (id) ON DELETE SET NULL,
    event_type                 TEXT NOT NULL,
    merchant_idempotency_key   TEXT NOT NULL,
    target_url                 TEXT NOT NULL,
    payload                    JSONB NOT NULL DEFAULT '{}',
    status                     TEXT NOT NULL DEFAULT 'pending',
    attempt_count              INT NOT NULL DEFAULT 0,
    max_attempts               INT NOT NULL DEFAULT 5,
    last_error                 TEXT,
    next_attempt_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, merchant_idempotency_key)
);

CREATE INDEX webhook_deliveries_tenant_status_idx ON webhook_deliveries (tenant_id, status);
