package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const apiKeyPrefix = "pf_live_"

// HashAPIKey returns a stable hex digest for storage and lookup.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// GenerateAPIKey returns a new raw secret suitable for X-API-Key.
func GenerateAPIKey() (raw string, err error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return apiKeyPrefix + hex.EncodeToString(b[:]), nil
}

// APIKeyHeader is the inbound header name for merchant integrations.
const APIKeyHeader = "X-Api-Key"

// MiddlewareAPIKey resolves X-Api-Key to tenant context (R8).
func MiddlewareAPIKey(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimSpace(r.Header.Get(APIKeyHeader))
			if raw == "" {
				http.Error(w, `{"error":"missing_api_key"}`, http.StatusUnauthorized)
				return
			}
			hash := HashAPIKey(raw)
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
			ctx := WithTenantPrincipal(r.Context(), tenantID, PrincipalAPIKey, keyID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ParseAPIKeyHash is used in tests and tooling.
func ParseAPIKeyHash(raw string) string {
	return HashAPIKey(raw)
}

// ErrInvalidAPIKey is returned by lookup helpers in tests.
var ErrInvalidAPIKey = fmt.Errorf("invalid api key")
