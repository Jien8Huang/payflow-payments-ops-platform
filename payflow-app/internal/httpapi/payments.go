package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/payflow/payflow-app/internal/auth"
	"github.com/payflow/payflow-app/internal/ledger"
	"github.com/payflow/payflow-app/internal/payment"
)

type createPaymentBody struct {
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

func (s *Server) postPayment(w http.ResponseWriter, r *http.Request) {
	tid, ok := requireTenantID(w, r)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		http.Error(w, `{"error":"missing_idempotency_key"}`, http.StatusBadRequest)
		return
	}
	var body createPaymentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}
	p, created, err := s.Payments.Create(r.Context(), tid, key, body.AmountCents, body.Currency)
	if err != nil {
		if errors.Is(err, payment.ErrIdempotencyMismatch) {
			http.Error(w, `{"error":"idempotency_conflict"}`, http.StatusConflict)
			return
		}
		if errors.Is(err, payment.ErrInvalidInput) {
			http.Error(w, `{"error":"invalid_payment"}`, http.StatusBadRequest)
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
	_ = json.NewEncoder(w).Encode(paymentJSONView(p))
}

func (s *Server) getPayment(w http.ResponseWriter, r *http.Request) {
	tid, ok := requireTenantID(w, r)
	if !ok {
		return
	}
	pid, err := uuid.Parse(chi.URLParam(r, "paymentID"))
	if err != nil {
		http.Error(w, `{"error":"invalid_payment_id"}`, http.StatusBadRequest)
		return
	}
	p, err := s.Payments.Get(r.Context(), tid, pid)
	if err != nil {
		if errors.Is(err, payment.ErrNotFound) {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"load_failed"}`, http.StatusInternalServerError)
		return
	}
	events, err := ledger.ListByPayment(r.Context(), s.Pool, tid, pid)
	if err != nil {
		http.Error(w, `{"error":"ledger_failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	out := paymentJSONView(p)
	out["ledger_events"] = ledgerEventsJSON(events)
	_ = json.NewEncoder(w).Encode(out)
}

func requireTenantID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	tid, ok := auth.TenantID(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return uuid.Nil, false
	}
	return tid, true
}

func paymentJSONView(p payment.Payment) map[string]any {
	return map[string]any{
		"id":                p.ID.String(),
		"tenant_id":         p.TenantID.String(),
		"amount_cents":      p.AmountCents,
		"currency":          p.Currency,
		"status":            p.Status,
		"idempotency_key":   p.IdempotencyKey,
		"idempotency_scope": p.IdempotencyScope,
		"created_at":        p.CreatedAt.Format(time.RFC3339Nano),
		"updated_at":        p.UpdatedAt.Format(time.RFC3339Nano),
	}
}

func ledgerEventsJSON(events []ledger.Event) []map[string]any {
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, map[string]any{
			"id":         e.ID,
			"dedupe_key": e.DedupeKey,
			"event_type": e.EventType,
			"payload":    e.Payload,
			"created_at": e.CreatedAt.Format(time.RFC3339Nano),
		})
	}
	return out
}
