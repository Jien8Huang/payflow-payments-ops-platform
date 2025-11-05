package tenant

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
	"github.com/payflow/payflow-app/internal/auth"
)

var ErrInvalidName = errors.New("invalid tenant name")

type Service struct {
	Pool *pgxpool.Pool
}

// ValidateTenantName checks onboarding input before hitting the database.
func ValidateTenantName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 128 {
		return ErrInvalidName
	}
	return nil
}

// CreateTenantWithAPIKey creates a tenant and its first API key (R1). Returns raw API key once.
func (s *Service) CreateTenantWithAPIKey(ctx context.Context, name string) (tenantID uuid.UUID, rawAPIKey string, err error) {
	if err := ValidateTenantName(name); err != nil {
		return uuid.Nil, "", err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tid uuid.UUID
	err = tx.QueryRow(ctx, `INSERT INTO tenants (name) VALUES ($1) RETURNING id`, strings.TrimSpace(name)).Scan(&tid)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("insert tenant: %w", err)
	}
	rawAPIKey, err = auth.GenerateAPIKey()
	if err != nil {
		return uuid.Nil, "", err
	}
	hash := auth.HashAPIKey(rawAPIKey)
	prefix := rawAPIKey
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_keys (tenant_id, key_hash, key_prefix)
		VALUES ($1, $2, $3)
	`, tid, hash, prefix)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("insert api key: %w", err)
	}
	if err := audit.Write(ctx, tx, &tid, "tenant_created", map[string]any{"name": strings.TrimSpace(name)}); err != nil {
		return uuid.Nil, "", err
	}
	if err := audit.Write(ctx, tx, &tid, "api_key_issued", map[string]any{"key_prefix": prefix}); err != nil {
		return uuid.Nil, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, "", err
	}
	return tid, rawAPIKey, nil
}

// APIKeyInfo is a non-secret view of an integration key.
type APIKeyInfo struct {
	ID        uuid.UUID
	KeyPrefix string
	CreatedAt time.Time
	RevokedAt *time.Time
}

// ListAPIKeys returns API keys for a tenant (R2 / R11 visibility).
func (s *Service) ListAPIKeys(ctx context.Context, tenantID uuid.UUID) ([]APIKeyInfo, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, key_prefix, created_at, revoked_at
		FROM api_keys WHERE tenant_id = $1 ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKeyInfo
	for rows.Next() {
		var k APIKeyInfo
		var rev *time.Time
		if err := rows.Scan(&k.ID, &k.KeyPrefix, &k.CreatedAt, &rev); err != nil {
			return nil, err
		}
		k.RevokedAt = rev
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokeAPIKey marks a key revoked if it belongs to the tenant.
func (s *Service) RevokeAPIKey(ctx context.Context, tenantID, keyID uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE api_keys SET revoked_at = now()
		WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL
	`, keyID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err := audit.Write(ctx, tx, &tenantID, "api_key_revoked", map[string]any{"key_id": keyID.String()}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetName returns tenant display name for the given id.
func (s *Service) GetName(ctx context.Context, tenantID uuid.UUID) (string, error) {
	var name string
	err := s.Pool.QueryRow(ctx, `SELECT name FROM tenants WHERE id = $1`, tenantID).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}
