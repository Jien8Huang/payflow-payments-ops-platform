package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/payflow/payflow-app/internal/audit"
	"github.com/payflow/payflow-app/internal/ledger"
	"github.com/payflow/payflow-app/internal/metrics"
	"github.com/payflow/payflow-app/internal/outbox"
	"github.com/payflow/payflow-app/internal/queue"
)

// ScopePaymentCreate is stored with idempotency_key (R4).
const ScopePaymentCreate = "payment:create"

const (
	LedgerPaymentAccepted     = "payment.accepted"
	LedgerSettlementCompleted = "settlement.completed"
)

var (
	ErrIdempotencyMismatch = errors.New("payment: idempotency key reused with different body")
	ErrInvalidInput        = errors.New("payment: invalid input")
	ErrNotFound            = errors.New("payment: not found")
)

// Payment is a persisted charge (mock settlement in Unit 5).
type Payment struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	AmountCents      int64
	Currency         string
	Status           string
	IdempotencyKey   string
	IdempotencyScope string
	RequestHash      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Service owns payment persistence and enqueue.
// Metrics: increment a `payments_created` counter (or equivalent) when Create returns created=true — wiring deferred to Unit 8.
type Service struct {
	Pool *pgxpool.Pool
	Q    queue.Publisher
}

func validateCreate(amountCents int64, currency, idempotencyKey string) error {
	if strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 200 {
		return fmt.Errorf("%w: idempotency key", ErrInvalidInput)
	}
	if amountCents <= 0 {
		return fmt.Errorf("%w: amount_cents", ErrInvalidInput)
	}
	cur := strings.TrimSpace(currency)
	if len(cur) != 3 {
		return fmt.Errorf("%w: currency", ErrInvalidInput)
	}
	return nil
}

// Create persists a payment under idempotency rules (first writer wins; mismatched body → 409).
func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, idempotencyKey string, amountCents int64, currency string) (p Payment, created bool, err error) {
	if err := validateCreate(amountCents, currency, idempotencyKey); err != nil {
		return Payment{}, false, err
	}
	cur := strings.ToUpper(strings.TrimSpace(currency))
	fp := RequestFingerprint(amountCents, cur)

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Payment{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id uuid.UUID
	var status, storedHash string
	var createdAt, updatedAt time.Time

	insert := `
		INSERT INTO payments (tenant_id, amount_cents, currency, status, idempotency_scope, idempotency_key, request_hash)
		VALUES ($1, $2, $3, 'pending', $4, $5, $6)
		ON CONFLICT (tenant_id, idempotency_scope, idempotency_key) DO NOTHING
		RETURNING id, status, request_hash, created_at, updated_at
	`
	err = tx.QueryRow(ctx, insert, tenantID, amountCents, cur, ScopePaymentCreate, idempotencyKey, fp).
		Scan(&id, &status, &storedHash, &createdAt, &updatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			SELECT id, status, request_hash, created_at, updated_at
			FROM payments
			WHERE tenant_id = $1 AND idempotency_scope = $2 AND idempotency_key = $3
		`, tenantID, ScopePaymentCreate, idempotencyKey).Scan(&id, &status, &storedHash, &createdAt, &updatedAt)
		if err != nil {
			return Payment{}, false, err
		}
		if storedHash != fp {
			return Payment{}, false, ErrIdempotencyMismatch
		}
		if err := tx.Commit(ctx); err != nil {
			return Payment{}, false, err
		}
		return Payment{
			ID: id, TenantID: tenantID, AmountCents: amountCents, Currency: cur, Status: status,
			IdempotencyKey: idempotencyKey, IdempotencyScope: ScopePaymentCreate, RequestHash: storedHash,
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		}, false, nil
	}
	if err != nil {
		return Payment{}, false, err
	}

	if _, err := ledger.Append(ctx, tx, tenantID, id, LedgerPaymentAccepted, LedgerPaymentAccepted, map[string]any{
		"amount_cents": amountCents,
		"currency":     cur,
	}); err != nil {
		return Payment{}, false, err
	}
	if err := audit.Write(ctx, tx, &tenantID, "payment_created", map[string]any{
		"payment_id": id.String(), "amount_cents": amountCents, "currency": cur,
	}); err != nil {
		return Payment{}, false, err
	}
	if err := outbox.InsertPaymentSettlement(ctx, tx, id); err != nil {
		return Payment{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Payment{}, false, err
	}
	metrics.PaymentsCreated.Inc()

	return Payment{
		ID: id, TenantID: tenantID, AmountCents: amountCents, Currency: cur, Status: status,
		IdempotencyKey: idempotencyKey, IdempotencyScope: ScopePaymentCreate, RequestHash: fp,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, true, nil
}

// Get returns a payment for the tenant (404-style not found on cross-tenant access).
func (s *Service) Get(ctx context.Context, tenantID, paymentID uuid.UUID) (Payment, error) {
	var p Payment
	err := s.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, amount_cents, currency, status, idempotency_scope, idempotency_key, request_hash, created_at, updated_at
		FROM payments WHERE id = $1 AND tenant_id = $2
	`, paymentID, tenantID).Scan(
		&p.ID, &p.TenantID, &p.AmountCents, &p.Currency, &p.Status,
		&p.IdempotencyScope, &p.IdempotencyKey, &p.RequestHash, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, ErrNotFound
		}
		return Payment{}, err
	}
	return p, nil
}

// SettleMockTx advances pending → succeeded using an existing transaction (outbox worker).
func SettleMockTx(ctx context.Context, tx pgx.Tx, paymentID uuid.UUID) error {
	var tenantID uuid.UUID
	var status string
	err := tx.QueryRow(ctx, `
		SELECT tenant_id, status FROM payments WHERE id = $1 FOR UPDATE
	`, paymentID).Scan(&tenantID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if status == "succeeded" {
		return nil
	}
	if _, err := ledger.Append(ctx, tx, tenantID, paymentID, LedgerSettlementCompleted, LedgerSettlementCompleted, map[string]any{
		"mock": true,
	}); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE payments SET status = 'succeeded', updated_at = now()
		WHERE id = $1 AND status = 'pending'
	`, paymentID)
	return err
}

// SettleMock advances pending → succeeded and appends settlement ledger (idempotent under redelivery).
func SettleMock(ctx context.Context, pool *pgxpool.Pool, paymentID uuid.UUID) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := SettleMockTx(ctx, tx, paymentID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
