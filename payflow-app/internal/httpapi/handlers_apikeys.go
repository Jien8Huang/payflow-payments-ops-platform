package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/payflow/payflow-app/internal/auth"
)

func (s *Server) getTenantAPIKeys(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantID(r.Context())
	if !ok {
		http.Error(w, `{"error":"no_tenant"}`, http.StatusUnauthorized)
		return
	}
	keys, err := s.Tenants.ListAPIKeys(r.Context(), tid)
	if err != nil {
		http.Error(w, `{"error":"list_failed"}`, http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		m := map[string]any{
			"id":         k.ID.String(),
			"key_prefix": k.KeyPrefix,
			"created_at": k.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if k.RevokedAt != nil {
			m["revoked_at"] = k.RevokedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		out = append(out, m)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"api_keys": out})
}

func (s *Server) deleteTenantAPIKey(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantID(r.Context())
	if !ok {
		http.Error(w, `{"error":"no_tenant"}`, http.StatusUnauthorized)
		return
	}
	kid, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		http.Error(w, `{"error":"invalid_id"}`, http.StatusBadRequest)
		return
	}
	if err := s.Tenants.RevokeAPIKey(r.Context(), tid, kid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"revoke_failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
