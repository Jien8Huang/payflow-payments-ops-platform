package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func SignDashboardToken(secret []byte, tenantID, userID uuid.UUID, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"tid": tenantID.String(),
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(secret)
}

func ParseDashboardBearer(secret []byte, authzHeader string) (tenantID, userID uuid.UUID, err error) {
	if authzHeader == "" {
		return uuid.Nil, uuid.Nil, errors.New("missing authorization")
	}
	const p = "Bearer "
	if !strings.HasPrefix(authzHeader, p) {
		return uuid.Nil, uuid.Nil, errors.New("not bearer")
	}
	raw := strings.TrimSpace(strings.TrimPrefix(authzHeader, p))
	tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !tok.Valid {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse jwt: %w", err)
	}
	mc, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, uuid.Nil, errors.New("invalid claims")
	}
	tidStr, ok := mc["tid"].(string)
	if !ok {
		return uuid.Nil, uuid.Nil, errors.New("missing tid")
	}
	subStr, ok := mc["sub"].(string)
	if !ok {
		return uuid.Nil, uuid.Nil, errors.New("missing sub")
	}
	tid, err := uuid.Parse(tidStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("tid: %w", err)
	}
	uid, err := uuid.Parse(subStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("sub: %w", err)
	}
	return tid, uid, nil
}

// MiddlewareBearerJWT validates Authorization: Bearer for dashboard routes (R2).
func MiddlewareBearerJWT(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tid, uid, err := ParseDashboardBearer(secret, r.Header.Get("Authorization"))
			if err != nil {
				http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
				return
			}
			ctx := WithTenantPrincipal(r.Context(), tid, PrincipalDashboard, uid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
