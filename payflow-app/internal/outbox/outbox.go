package outbox

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	KindPaymentSettlement = "payment_settlement"
	KindRefundSettlement  = "refund_settlement"
)

// Execer matches pgx.Tx and pgxpool.Pool for INSERTs.
type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func InsertPaymentSettlement(ctx context.Context, q Execer, paymentID uuid.UUID) error {
	b, err := json.Marshal(map[string]string{"payment_id": paymentID.String()})
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO async_outbox (kind, payload) VALUES ($1, $2::jsonb)`, KindPaymentSettlement, b)
	return err
}

func InsertRefundSettlement(ctx context.Context, q Execer, refundID uuid.UUID) error {
	b, err := json.Marshal(map[string]string{"refund_id": refundID.String()})
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO async_outbox (kind, payload) VALUES ($1, $2::jsonb)`, KindRefundSettlement, b)
	return err
}
