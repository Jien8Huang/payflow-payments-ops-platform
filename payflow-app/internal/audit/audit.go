package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// Execer matches *pgxpool.Pool and pgx.Tx for transactional audit writes.
type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// Write persists an audit row (R11). event_type must be stable across releases.
func Write(ctx context.Context, q Execer, tenantID *uuid.UUID, eventType string, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		INSERT INTO audit_events (tenant_id, event_type, metadata)
		VALUES ($1, $2, $3::jsonb)
	`, tenantID, eventType, b)
	return err
}
