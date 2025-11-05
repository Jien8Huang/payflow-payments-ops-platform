-- Transactional async outbox (docs/contracts/async-plane.md): enqueue with business writes.
CREATE TABLE IF NOT EXISTS async_outbox (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind         TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT async_outbox_kind_check CHECK (kind IN ('payment_settlement', 'refund_settlement'))
);

CREATE INDEX IF NOT EXISTS async_outbox_pending_idx ON async_outbox (created_at) WHERE processed_at IS NULL;
