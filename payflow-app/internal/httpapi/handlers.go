package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/payflow/payflow-app/internal/audit"
	"github.com/payflow/payflow-app/internal/auth"
	"github.com/payflow/payflow-app/internal/payment"
	"github.com/payflow/payflow-app/internal/queue"
	"github.com/payflow/payflow-app/internal/refund"
	"github.com/payflow/payflow-app/internal/tenant"
)

// Server wires HTTP handlers.
type Server struct {
	Pool      *pgxpool.Pool
	Tenants   *tenant.Service
	Payments  *payment.Service
	Refunds   *refund.Service
	Pub       queue.Publisher // webhook DLQ replay and legacy Redis enqueue paths
	JWTSecret []byte
}

func (s *Server) postTenants(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}
	tid, key, err := s.Tenants.CreateTenantWithAPIKey(r.Context(), body.Name)
	if err != nil {
		if err == tenant.ErrInvalidName {
			http.Error(w, `{"error":"invalid_tenant_name"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"create_failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tenant_id": tid.String(),
		"api_key":   key,
	})
}

func (s *Server) getTenantsMe(w http.ResponseWriter, r *http.Request) {
	tid, ok := auth.TenantID(r.Context())
	if !ok {
		http.Error(w, `{"error":"no_tenant"}`, http.StatusUnauthorized)
		return
	}
	name, err := s.Tenants.GetName(r.Context(), tid)
	if err != nil {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}
	pk, _ := auth.Principal(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tenant_id": tid.String(),
		"name":      name,
		"principal": string(pk),
	})
}

type dashboardUserBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) postDashboardUser(w http.ResponseWriter, r *http.Request) {
	ctxTid, ok := auth.TenantID(r.Context())
	if !ok {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	pk, ok := auth.Principal(r.Context())
	if !ok || pk != auth.PrincipalAPIKey {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	pathID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil || pathID != ctxTid {
		http.Error(w, `{"error":"tenant_mismatch"}`, http.StatusForbidden)
		return
	}
	var body dashboardUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))
	if email == "" || len(body.Password) < 10 {
		http.Error(w, `{"error":"invalid_credentials"}`, http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error":"hash_failed"}`, http.StatusInternalServerError)
		return
	}
	var uid uuid.UUID
	err = s.Pool.QueryRow(r.Context(), `
		INSERT INTO dashboard_users (tenant_id, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id
	`, ctxTid, email, string(hash)).Scan(&uid)
	if err != nil {
		http.Error(w, `{"error":"user_create_failed"}`, http.StatusConflict)
		return
	}
	_ = audit.Write(r.Context(), s.Pool, &ctxTid, "dashboard_user_created", map[string]any{"email": email, "user_id": uid.String()})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"user_id": uid.String()})
}

type loginBody struct {
	TenantID uuid.UUID `json:"tenant_id"`
	Email    string    `json:"email"`
	Password string    `json:"password"`
}

func (s *Server) postAuthLogin(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))
	var uid uuid.UUID
	var hash string
	err := s.Pool.QueryRow(r.Context(), `
		SELECT id, password_hash FROM dashboard_users
		WHERE tenant_id = $1 AND email = $2
	`, body.TenantID, email).Scan(&uid, &hash)
	if err != nil {
		_ = audit.Write(r.Context(), s.Pool, &body.TenantID, "dashboard_login_failure", map[string]any{"email": email, "reason": "unknown_user"})
		http.Error(w, `{"error":"invalid_credentials"}`, http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		_ = audit.Write(r.Context(), s.Pool, &body.TenantID, "dashboard_login_failure", map[string]any{"email": email, "reason": "bad_password"})
		http.Error(w, `{"error":"invalid_credentials"}`, http.StatusUnauthorized)
		return
	}
	tok, err := auth.SignDashboardToken(s.JWTSecret, body.TenantID, uid, 24*time.Hour)
	if err != nil {
		http.Error(w, `{"error":"token_failed"}`, http.StatusInternalServerError)
		return
	}
	_ = audit.Write(r.Context(), s.Pool, &body.TenantID, "dashboard_login_success", map[string]any{"email": email, "user_id": uid.String()})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": tok, "token_type": "Bearer", "expires_in": 86400})
}
