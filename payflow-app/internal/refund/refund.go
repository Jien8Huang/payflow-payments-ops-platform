package refund

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/payflow/payflow-app/internal/audit"
	"github.com/payflow/payflow-app/internal/ledger"
	"github.com/payflow/payflow-app/internal/queue"
)

// ScopeRefundCreate is stored with idempotency_key (R4).
const ScopeRefundCreate = "refund:create"

const (
	LedgerRefundRequested = "refund.requested"
	LedgerRefundSucceeded = "refund.succeeded"
)

var (
	ErrIdempotencyMismatch  = errors.New("refund: idempotency key reused with different body")
	ErrInvalidInput         = errors.New("refund: invalid input")
	ErrNotFound             = errors.New("refund: not found")
	ErrPaymentNotRefundable = errors.New("refund: payment is not in a refundable state")
	ErrRefundAmountExceeded = errors.New("refund: amount exceeds remaining refundable balance")
)

// Refund is a persisted refund request (mock completion via worker).
type Refund struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	PaymentID        uuid.UUID
	AmountCents      int64
	Currency         string
	Status           string
	IdempotencyKey   string
	IdempotencyScope string
	RequestHash      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type createFingerprint struct {
	PaymentID   string `json:"payment_id"`
	AmountCents int64  `json:"amount_cents"`
}

func requestFingerprint(paymentID uuid.UUID, amountCents int64) string {
	b, _ := json.Marshal(createFingerprint{PaymentID: paymentID.String(), AmountCents: amountCents})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Service owns refund persistence.
type Service struct {
	Pool *pgxpool.Pool
	Q    queue.Publisher
}

// Create records a refund for a succeeded payment (full or partial). amount_cents 0 means full payment amount.
func (s *Service) Create(ctx context.Context, tenantID, paymentID uuid.UUID, idempotencyKey string, amountCents int64) (r Refund, created bool, err error) {
	if strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 200 {
		return Refund{}, false, fmt.Errorf("%w: idempotency key", ErrInvalidInput)
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Refund{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var payTenant uuid.UUID
	var payStatus string
	var payAmount int64
	var cur string
	err = tx.QueryRow(ctx, `
		SELECT tenant_id, status, amount_cents, currency FROM payments WHERE id = $1 FOR UPDATE
	`, paymentID).Scan(&payTenant, &payStatus, &payAmount, &cur)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Refund{}, false, ErrNotFound
		}
		return Refund{}, false, err
	}
	if payTenant != tenantID {
		return Refund{}, false, ErrNotFound
	}
	if payStatus != "succeeded" {
		return Refund{}, false, ErrPaymentNotRefundable
	}

	amt := amountCents
	if amt == 0 {
		amt = payAmount
	}
	if amt <= 0 || amt > payAmount {
		return Refund{}, false, ErrInvalidInput
	}

	var reserved int64
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM refunds
		WHERE payment_id = $1 AND status IN ('pending', 'succeeded')
	`, paymentID).Scan(&reserved)
	if err != nil {
		return Refund{}, false, err
	}
	if reserved+amt > payAmount {
		return Refund{}, false, ErrRefundAmountExceeded
	}

	fp := requestFingerprint(paymentID, amt)
	cur = strings.ToUpper(strings.TrimSpace(cur))

	var id uuid.UUID
	var status, storedHash string
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO refunds (tenant_id, payment_id, amount_cents, currency, status, idempotency_scope, idempotency_key, request_hash)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7)
		ON CONFLICT (tenant_id, idempotency_scope, idempotency_key) DO NOTHING
		RETURNING id, status, request_hash, created_at, updated_at
	`, tenantID, paymentID, amt, cur, ScopeRefundCreate, idempotencyKey, fp).
		Scan(&id, &status, &storedHash, &createdAt, &updatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			SELECT id, status, request_hash, created_at, updated_at
			FROM refunds WHERE tenant_id = $1 AND idempotency_scope = $2 AND idempotency_key = $3
		`, tenantID, ScopeRefundCreate, idempotencyKey).Scan(&id, &status, &storedHash, &createdAt, &updatedAt)
		if err != nil {
			return Refund{}, false, err
		}
		if storedHash != fp {
			return Refund{}, false, ErrIdempotencyMismatch
		}
		if err := tx.Commit(ctx); err != nil {
			return Refund{}, false, err
		}
		return Refund{
			ID: id, TenantID: tenantID, PaymentID: paymentID, AmountCents: amt, Currency: cur, Status: status,
			IdempotencyKey: idempotencyKey, IdempotencyScope: ScopeRefundCreate, RequestHash: storedHash,
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		}, false, nil
	}
	if err != nil {
		return Refund{}, false, err
	}

	if _, err := ledger.Append(ctx, tx, tenantID, paymentID, "refund:"+id.String()+":requested", LedgerRefundRequested, map[string]any{
		"refund_id": id.String(), "amount_cents": amt, "currency": cur,
	}); err != nil {
		return Refund{}, false, err
	}
	if err := audit.Write(ctx, tx, &tenantID, "refund_created", map[string]any{
		"refund_id": id.String(), "payment_id": paymentID.String(), "amount_cents": amt,
	}); err != nil {
		return Refund{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Refund{}, false, err
	}

	if err := s.Q.PublishRefundSettlement(ctx, id); err != nil {
		return Refund{
			ID: id, TenantID: tenantID, PaymentID: paymentID, AmountCents: amt, Currency: cur, Status: status,
			IdempotencyKey: idempotencyKey, IdempotencyScope: ScopeRefundCreate, RequestHash: fp,
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		}, true, fmt.Errorf("enqueue refund settlement: %w", err)
	}

	return Refund{
		ID: id, TenantID: tenantID, PaymentID: paymentID, AmountCents: amt, Currency: cur, Status: status,
		IdempotencyKey: idempotencyKey, IdempotencyScope: ScopeRefundCreate, RequestHash: fp,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, true, nil
}

// Get returns a refund scoped to tenant.
func (s *Service) Get(ctx context.Context, tenantID, refundID uuid.UUID) (Refund, error) {
	var r Refund
	err := s.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, payment_id, amount_cents, currency, status, idempotency_scope, idempotency_key, request_hash, created_at, updated_at
		FROM refunds WHERE id = $1 AND tenant_id = $2
	`, refundID, tenantID).Scan(
		&r.ID, &r.TenantID, &r.PaymentID, &r.AmountCents, &r.Currency, &r.Status,
		&r.IdempotencyScope, &r.IdempotencyKey, &r.RequestHash, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Refund{}, ErrNotFound
		}
		return Refund{}, err
	}
	return r, nil
}

// SettleMock marks a pending refund succeeded and appends ledger (idempotent).
func SettleMock(ctx context.Context, pool *pgxpool.Pool, refundID uuid.UUID) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tenantID, paymentID uuid.UUID
	var rstatus, pstatus string
	err = tx.QueryRow(ctx, `
		SELECT r.tenant_id, r.payment_id, r.status, p.status
		FROM refunds r JOIN payments p ON p.id = r.payment_id
		WHERE r.id = $1 FOR UPDATE OF r, p
	`, refundID).Scan(&tenantID, &paymentID, &rstatus, &pstatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tx.Commit(ctx)
		}
		return err
	}
	if rstatus != "pending" {
		return tx.Commit(ctx)
	}
	if pstatus != "succeeded" {
		return fmt.Errorf("refund settle: payment %s status %q", paymentID, pstatus)
	}
	if _, err := ledger.Append(ctx, tx, tenantID, paymentID, "refund:"+refundID.String()+":succeeded", LedgerRefundSucceeded, map[string]any{
		"refund_id": refundID.String(),
	}); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		UPDATE refunds SET status = 'succeeded', updated_at = now()
		WHERE id = $1 AND status = 'pending'
	`, refundID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
