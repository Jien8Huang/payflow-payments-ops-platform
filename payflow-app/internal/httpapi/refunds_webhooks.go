package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/payflow/payflow-app/internal/refund"
	"github.com/payflow/payflow-app/internal/webhook"
)

type patchWebhookBody struct {
	URL           string `json:"url"`
	SigningSecret string `json:"signing_secret"`
}

func (s *Server) patchTenantWebhook(w http.ResponseWriter, r *http.Request) {
	tid, ok := requireTenantID(w, r)
	if !ok {
		return
	}
	var body patchWebhookBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}
	url := strings.TrimSpace(body.URL)
	if url == "" {
		http.Error(w, `{"error":"missing_url"}`, http.StatusBadRequest)
		return
	}
	_, err := s.Pool.Exec(r.Context(), `
		UPDATE tenants SET webhook_url = $2, webhook_signing_secret = $3 WHERE id = $1
	`, tid, url, body.SigningSecret)
	if err != nil {
		http.Error(w, `{"error":"update_failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) listWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	tid, ok := requireTenantID(w, r)
	if !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	rows, err := webhook.ListDeliveries(r.Context(), s.Pool, tid, status, 50)
	if err != nil {
		http.Error(w, `{"error":"list_failed"}`, http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":            row.ID.String(),
			"event_type":    row.EventType,
			"status":        row.Status,
			"attempt_count": row.AttemptCount,
			"max_attempts":  row.MaxAttempts,
			"last_error":    row.LastError,
			"created_at":    row.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"deliveries": out})
}

func (s *Server) getWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	tid, ok := requireTenantID(w, r)
	if !ok {
		return
	}
	did, err := uuid.Parse(chi.URLParam(r, "deliveryID"))
	if err != nil {
		http.Error(w, `{"error":"invalid_id"}`, http.StatusBadRequest)
		return
	}
	d, err := webhook.GetDelivery(r.Context(), s.Pool, tid, did)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"load_failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	m := map[string]any{
		"id":                       d.ID.String(),
		"tenant_id":                d.TenantID.String(),
		"event_type":               d.EventType,
		"merchant_idempotency_key": d.MerchantIDempotencyKey,
		"target_url":               d.TargetURL,
		"payload":                  d.Payload,
		"status":                   d.Status,
		"attempt_count":            d.AttemptCount,
		"max_attempts":             d.MaxAttempts,
		"last_error":               d.LastError,
		"created_at":               d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":               d.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if d.PaymentID.Valid {
		m["payment_id"] = d.PaymentID.String
	}
	if d.RefundID.Valid {
		m["refund_id"] = d.RefundID.String
	}
	_ = json.NewEncoder(w).Encode(m)
}

// postWebhookDeliveryRetry re-queues a DLQ delivery (R6 optional replay).
func (s *Server) postWebhookDeliveryRetry(w http.ResponseWriter, r *http.Request) {
	tid, ok := requireTenantID(w, r)
	if !ok {
		return
	}
	did, err := uuid.Parse(chi.URLParam(r, "deliveryID"))
	if err != nil {
		http.Error(w, `{"error":"invalid_id"}`, http.StatusBadRequest)
		return
	}
	var status string
	err = s.Pool.QueryRow(r.Context(), `
		SELECT status FROM webhook_deliveries WHERE id = $1 AND tenant_id = $2
	`, did, tid).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"load_failed"}`, http.StatusInternalServerError)
		return
	}
	if status != "dlq" {
		http.Error(w, `{"error":"not_dlq"}`, http.StatusBadRequest)
		return
	}
	if _, err := s.Pool.Exec(r.Context(), `
		UPDATE webhook_deliveries
		SET status = 'pending', attempt_count = 0, last_error = '', next_attempt_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND status = 'dlq'
	`, did, tid); err != nil {
		http.Error(w, `{"error":"update_failed"}`, http.StatusInternalServerError)
		return
	}
	if err := s.Pub.PublishWebhookDelivery(r.Context(), did); err != nil {
		http.Error(w, `{"error":"enqueue_failed"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "delivery_id": did.String()})
}

type postRefundBody struct {
	AmountCents int64 `json:"amount_cents"` // 0 = full payment amount
}

func (s *Server) postRefund(w http.ResponseWriter, r *http.Request) {
	tid, ok := requireTenantID(w, r)
	if !ok {
		return
	}
	pid, err := uuid.Parse(chi.URLParam(r, "paymentID"))
	if err != nil {
		http.Error(w, `{"error":"invalid_payment_id"}`, http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		http.Error(w, `{"error":"missing_idempotency_key"}`, http.StatusBadRequest)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"invalid_body"}`, http.StatusBadRequest)
		return
	}
	var body postRefundBody
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
			return
		}
	}
	ref, created, err := s.Refunds.Create(r.Context(), tid, pid, key, body.AmountCents)
	if err != nil {
		if errors.Is(err, refund.ErrIdempotencyMismatch) {
			http.Error(w, `{"error":"idempotency_conflict"}`, http.StatusConflict)
			return
		}
		if errors.Is(err, refund.ErrInvalidInput) || errors.Is(err, refund.ErrRefundAmountExceeded) {
			http.Error(w, `{"error":"invalid_refund"}`, http.StatusBadRequest)
			return
		}
		if errors.Is(err, refund.ErrPaymentNotRefundable) {
			http.Error(w, `{"error":"payment_not_refundable"}`, http.StatusConflict)
			return
		}
		if errors.Is(err, refund.ErrNotFound) {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"create_failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(refundJSON(ref))
}

func (s *Server) getRefund(w http.ResponseWriter, r *http.Request) {
	tid, ok := requireTenantID(w, r)
	if !ok {
		return
	}
	rid, err := uuid.Parse(chi.URLParam(r, "refundID"))
	if err != nil {
		http.Error(w, `{"error":"invalid_refund_id"}`, http.StatusBadRequest)
		return
	}
	ref, err := s.Refunds.Get(r.Context(), tid, rid)
	if err != nil {
		if errors.Is(err, refund.ErrNotFound) {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"load_failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(refundJSON(ref))
}

func refundJSON(r refund.Refund) map[string]any {
	return map[string]any{
		"id":                r.ID.String(),
		"tenant_id":         r.TenantID.String(),
		"payment_id":        r.PaymentID.String(),
		"amount_cents":      r.AmountCents,
		"currency":          r.Currency,
		"status":            r.Status,
		"idempotency_key":   r.IdempotencyKey,
		"idempotency_scope": r.IdempotencyScope,
		"created_at":        r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":        r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
