package ledger

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Execer matches *pgxpool.Pool and pgx.Tx.
type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// Append inserts one append-only row; duplicate dedupe_key is a no-op (R13).
func Append(ctx context.Context, q Execer, tenantID, paymentID uuid.UUID, dedupeKey, eventType string, payload map[string]any) (inserted bool, err error) {
	if payload == nil {
		payload = map[string]any{}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	tag, err := q.Exec(ctx, `
		INSERT INTO ledger_events (tenant_id, payment_id, dedupe_key, event_type, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (payment_id, dedupe_key) DO NOTHING
	`, tenantID, paymentID, dedupeKey, eventType, b)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListByPayment returns events for a payment in monotonic id order (tenant enforced).
func ListByPayment(ctx context.Context, pool *pgxpool.Pool, tenantID, paymentID uuid.UUID) ([]Event, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, dedupe_key, event_type, payload, created_at
		FROM ledger_events
		WHERE tenant_id = $1 AND payment_id = $2
		ORDER BY id ASC
	`, tenantID, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var payload []byte
		if err := rows.Scan(&e.ID, &e.DedupeKey, &e.EventType, &payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &e.Payload); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Event is one persisted ledger row.
type Event struct {
	ID        int64
	DedupeKey string
	EventType string
	Payload   map[string]any
	CreatedAt time.Time
}
