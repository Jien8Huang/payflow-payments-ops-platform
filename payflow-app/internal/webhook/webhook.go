package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/payflow/payflow-app/internal/queue"
)

const (
	EventPaymentSettled  = "payment.settled"
	EventRefundSucceeded = "refund.succeeded"
	headerTimestamp      = "X-Payflow-Timestamp"
	headerSignature      = "X-Payflow-Signature"
)

// Delivery is one outbound HTTP notification.
type Delivery struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	PaymentID              sql.NullString
	RefundID               sql.NullString
	EventType              string
	MerchantIDempotencyKey string
	TargetURL              string
	Payload                map[string]any
	Status                 string
	AttemptCount           int
	MaxAttempts            int
	LastError              string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// ListRow is a truncated row for GET list.
type ListRow struct {
	ID           uuid.UUID
	EventType    string
	Status       string
	AttemptCount int
	MaxAttempts  int
	LastError    string
	CreatedAt    time.Time
}

func sign(secret, timestamp, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write([]byte(timestamp + "." + body))
	return hex.EncodeToString(m.Sum(nil))
}

// SignatureHex exposes the merchant HMAC for tests and tooling.
func SignatureHex(secret, timestamp, body string) string {
	return sign(secret, timestamp, body)
}

// EnqueuePaymentSettledIfNeeded inserts a delivery row (deduped) and publishes when newly created.
func EnqueuePaymentSettledIfNeeded(ctx context.Context, pool *pgxpool.Pool, pub queue.Publisher, paymentID uuid.UUID) error {
	var tenantID uuid.UUID
	var amount int64
	var cur string
	var hookURL, sec sql.NullString
	err := pool.QueryRow(ctx, `
		SELECT p.tenant_id, p.amount_cents, p.currency, t.webhook_url, t.webhook_signing_secret
		FROM payments p
		JOIN tenants t ON t.id = p.tenant_id
		WHERE p.id = $1 AND p.status = 'succeeded'
	`, paymentID).Scan(&tenantID, &amount, &cur, &hookURL, &sec)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !hookURL.Valid || hookURL.String == "" || !sec.Valid || sec.String == "" {
		return nil
	}

	merchantKey := "payflow/evt/payment.settled/" + paymentID.String()
	payload := map[string]any{
		"type":         EventPaymentSettled,
		"payment_id":   paymentID.String(),
		"amount_cents": amount,
		"currency":     cur,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var id uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO webhook_deliveries (
			tenant_id, payment_id, event_type, merchant_idempotency_key, target_url, payload, status, max_attempts
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'pending', 5)
		ON CONFLICT (tenant_id, merchant_idempotency_key) DO NOTHING
		RETURNING id
	`, tenantID, paymentID, EventPaymentSettled, merchantKey, hookURL.String, b).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return pub.PublishWebhookDelivery(ctx, id)
}

// EnqueueRefundSucceededIfNeeded queues a refund.succeeded webhook after the refund row is succeeded.
func EnqueueRefundSucceededIfNeeded(ctx context.Context, pool *pgxpool.Pool, pub queue.Publisher, refundID uuid.UUID) error {
	var tenantID, paymentID uuid.UUID
	var amount int64
	var cur string
	var hookURL, sec sql.NullString
	err := pool.QueryRow(ctx, `
		SELECT r.tenant_id, r.payment_id, r.amount_cents, r.currency, t.webhook_url, t.webhook_signing_secret
		FROM refunds r
		JOIN tenants t ON t.id = r.tenant_id
		WHERE r.id = $1 AND r.status = 'succeeded'
	`, refundID).Scan(&tenantID, &paymentID, &amount, &cur, &hookURL, &sec)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !hookURL.Valid || hookURL.String == "" || !sec.Valid || sec.String == "" {
		return nil
	}

	merchantKey := "payflow/evt/refund.succeeded/" + refundID.String()
	payload := map[string]any{
		"type":         EventRefundSucceeded,
		"refund_id":    refundID.String(),
		"payment_id":   paymentID.String(),
		"amount_cents": amount,
		"currency":     cur,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var id uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO webhook_deliveries (
			tenant_id, payment_id, refund_id, event_type, merchant_idempotency_key, target_url, payload, status, max_attempts
		) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, 'pending', 5)
		ON CONFLICT (tenant_id, merchant_idempotency_key) DO NOTHING
		RETURNING id
	`, tenantID, paymentID, refundID, EventRefundSucceeded, merchantKey, hookURL.String, b).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return pub.PublishWebhookDelivery(ctx, id)
}

// ProcessDelivery performs bounded HTTP attempts with exponential backoff between tries.
func ProcessDelivery(ctx context.Context, pool *pgxpool.Pool, client *http.Client, deliveryID uuid.UUID, defaultMaxAttempts int) error {
	if defaultMaxAttempts <= 0 {
		defaultMaxAttempts = 5
	}
	for {
		var targetURL, secret string
		var attemptCount, maxAttempts int
		var payload []byte
		var status string
		err := pool.QueryRow(ctx, `
			SELECT d.target_url, t.webhook_signing_secret, d.attempt_count, d.max_attempts, d.payload::text, d.status
			FROM webhook_deliveries d
			JOIN tenants t ON t.id = d.tenant_id
			WHERE d.id = $1
		`, deliveryID).Scan(&targetURL, &secret, &attemptCount, &maxAttempts, &payload, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if status != "pending" {
			return nil
		}
		if maxAttempts <= 0 {
			maxAttempts = defaultMaxAttempts
		}
		if attemptCount >= maxAttempts {
			_, _ = pool.Exec(ctx, `UPDATE webhook_deliveries SET status = 'dlq', updated_at = now() WHERE id = $1 AND status = 'pending'`, deliveryID)
			return nil
		}
		if secret == "" {
			_, _ = pool.Exec(ctx, `UPDATE webhook_deliveries SET status = 'dlq', last_error = $2, updated_at = now() WHERE id = $1 AND status = 'pending'`,
				deliveryID, "missing_webhook_signing_secret")
			return nil
		}

		ts := strconv.FormatInt(time.Now().Unix(), 10)
		body := string(payload)
		sig := sign(secret, ts, body)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader([]byte(body)))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(headerTimestamp, ts)
		req.Header.Set(headerSignature, sig)

		resp, err := client.Do(req)
		var errMsg string
		ok := false
		if err != nil {
			errMsg = err.Error()
		} else {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				ok = true
			} else {
				errMsg = fmt.Sprintf("http_%d", resp.StatusCode)
			}
		}

		if ok {
			_, err = pool.Exec(ctx, `UPDATE webhook_deliveries SET status = 'succeeded', updated_at = now() WHERE id = $1 AND status = 'pending'`, deliveryID)
			return err
		}

		newAttempts := attemptCount + 1
		newStatus := "pending"
		if newAttempts >= maxAttempts {
			newStatus = "dlq"
		}
		_, err = pool.Exec(ctx, `
			UPDATE webhook_deliveries
			SET attempt_count = $1, last_error = $2, status = $3, updated_at = now()
			WHERE id = $4 AND status = 'pending'
		`, newAttempts, errMsg, newStatus, deliveryID)
		if err != nil {
			return err
		}
		if newStatus == "dlq" {
			return nil
		}
		backoff := time.Duration(50*(1<<min(newAttempts, 6))) * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ListDeliveries returns recent deliveries for a tenant, optionally filtered by status (e.g. dlq).
func ListDeliveries(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, status string, limit int) ([]ListRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `
		SELECT id, event_type, status, attempt_count, max_attempts, coalesce(last_error,''), created_at
		FROM webhook_deliveries
		WHERE tenant_id = $1
	`
	args := []any{tenantID}
	if status != "" {
		q += ` AND status = $2`
		args = append(args, status)
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, len(args))

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ListRow
	for rows.Next() {
		var r ListRow
		if err := rows.Scan(&r.ID, &r.EventType, &r.Status, &r.AttemptCount, &r.MaxAttempts, &r.LastError, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetDelivery returns one delivery if it belongs to the tenant.
func GetDelivery(ctx context.Context, pool *pgxpool.Pool, tenantID, deliveryID uuid.UUID) (Delivery, error) {
	var d Delivery
	var payload []byte
	err := pool.QueryRow(ctx, `
		SELECT id, tenant_id, payment_id::text, refund_id::text, event_type, merchant_idempotency_key, target_url, payload::text,
		       status, attempt_count, max_attempts, coalesce(last_error,''), created_at, updated_at
		FROM webhook_deliveries
		WHERE id = $1 AND tenant_id = $2
	`, deliveryID, tenantID).Scan(
		&d.ID, &d.TenantID, &d.PaymentID, &d.RefundID, &d.EventType, &d.MerchantIDempotencyKey, &d.TargetURL, &payload,
		&d.Status, &d.AttemptCount, &d.MaxAttempts, &d.LastError, &d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Delivery{}, pgx.ErrNoRows
	}
	if err != nil {
		return Delivery{}, err
	}
	if err := json.Unmarshal(payload, &d.Payload); err != nil {
		d.Payload = map[string]any{"raw": string(payload)}
	}
	return d, nil
}
