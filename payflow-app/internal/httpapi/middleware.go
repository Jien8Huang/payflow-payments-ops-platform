package httpapi

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/payflow/payflow-app/internal/audit"
	"github.com/payflow/payflow-app/internal/auth"
)

// RequestID attaches a stable correlation id (R22).
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		w.Header().Set("X-Request-Id", id)
		ctx := auth.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MeAuth accepts either X-Api-Key or Bearer JWT for /v1/tenants/me (R2).
func MeAuth(pool *pgxpool.Pool, jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if raw := strings.TrimSpace(r.Header.Get(auth.APIKeyHeader)); raw != "" {
				hash := auth.HashAPIKey(raw)
				var tenantID, keyID uuid.UUID
				err := pool.QueryRow(r.Context(), `
					SELECT ak.tenant_id, ak.id
					FROM api_keys ak
					WHERE ak.key_hash = $1 AND ak.revoked_at IS NULL
				`, hash).Scan(&tenantID, &keyID)
				if err != nil {
					http.Error(w, `{"error":"invalid_api_key"}`, http.StatusUnauthorized)
					return
				}
				ctx := auth.WithTenantPrincipal(r.Context(), tenantID, auth.PrincipalAPIKey, keyID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			if authz := r.Header.Get("Authorization"); strings.HasPrefix(authz, "Bearer ") {
				tid, uid, err := auth.ParseDashboardBearer(jwtSecret, authz)
				if err != nil {
					_ = audit.Write(r.Context(), pool, nil, "dashboard_login_failure", map[string]any{
						"reason": "invalid_token",
						"path":   r.URL.Path,
					})
					http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
					return
				}
				ctx := auth.WithTenantPrincipal(r.Context(), tid, auth.PrincipalDashboard, uid)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		})
	}
}
