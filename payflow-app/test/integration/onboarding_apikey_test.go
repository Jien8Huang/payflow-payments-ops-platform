package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/payflow/payflow-app/internal/auth"
	"github.com/payflow/payflow-app/internal/config"
	"github.com/payflow/payflow-app/internal/db"
	"github.com/payflow/payflow-app/internal/migrate"
	"github.com/payflow/payflow-app/internal/queue"
)

func TestOnboardingAndIsolation(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 and start Postgres (e.g. docker compose) to run this test")
	}
	t.Setenv("JWT_SECRET", "integration-test-jwt-secret")

	ctx := context.Background()
	cfg := config.Load()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if os.Getenv("INTEGRATION_RESET") == "1" {
		resetPayflowTables(t, pool)
	}
	if err := migrate.Up(ctx, pool); err != nil {
		if os.Getenv("INTEGRATION_RESET") == "" {
			t.Fatalf("migrate failed: %v (use a fresh database or set INTEGRATION_RESET=1 to drop payflow tables first)", err)
		}
		t.Fatal(err)
	}

	ts := testHTTPServer(t, pool, queue.NoOpPublisher{})
	t.Cleanup(ts.Close)

	a := mustCreateTenant(t, ts.URL, "Tenant A Integration")
	b := mustCreateTenant(t, ts.URL, "Tenant B Integration")

	if !hasAudit(t, pool, a.TenantID, "api_key_issued") {
		t.Fatal("expected api_key_issued audit for tenant A")
	}

	meA := mustGETMe(t, ts.URL, a.APIKey)
	if meA["tenant_id"] != a.TenantID {
		t.Fatalf("/tenants/me: got %v", meA["tenant_id"])
	}

	meB := getMe(t, ts.URL, b.APIKey)
	if meB["tenant_id"] == a.TenantID {
		t.Fatal("tenant B key must not resolve to tenant A")
	}

	// Cross-tenant path: A's key against B's tenant id in URL → 403
	url := ts.URL + "/v1/tenants/" + b.TenantID + "/dashboard-users"
	body := []byte(`{"email":"ops@example.com","password":"longpassword1"}`)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.APIKeyHeader, a.APIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-tenant dashboard-user create: status %d", res.StatusCode)
	}

	// Happy path: create dashboard user for tenant A with A's key
	okURL := ts.URL + "/v1/tenants/" + a.TenantID + "/dashboard-users"
	req2, _ := http.NewRequest(http.MethodPost, okURL, bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(auth.APIKeyHeader, a.APIKey)
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusCreated {
		bb, _ := io.ReadAll(res2.Body)
		t.Fatalf("dashboard user create: status %d body %s", res2.StatusCode, bb)
	}

	beforeInvalid := invalidTokenAuditCount(t, pool)
	req3, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/tenants/me", nil)
	req3.Header.Set("Authorization", "Bearer not-a-jwt")
	res3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	_ = res3.Body.Close()
	if res3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid jwt /me: status %d", res3.StatusCode)
	}
	afterInvalid := invalidTokenAuditCount(t, pool)
	if afterInvalid <= beforeInvalid {
		t.Fatal("expected dashboard_login_failure audit for invalid JWT on /v1/tenants/me")
	}

	loginBody := map[string]any{
		"tenant_id": a.TenantID,
		"email":     "ops@example.com",
		"password":  "wrong-password",
	}
	lb, _ := json.Marshal(loginBody)
	req4, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/auth/login", bytes.NewReader(lb))
	req4.Header.Set("Content-Type", "application/json")
	beforeLoginFail := loginFailureAuditCount(t, pool, a.TenantID)
	res4, err := http.DefaultClient.Do(req4)
	if err != nil {
		t.Fatal(err)
	}
	_ = res4.Body.Close()
	if res4.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login: status %d", res4.StatusCode)
	}
	afterLoginFail := loginFailureAuditCount(t, pool, a.TenantID)
	if afterLoginFail <= beforeLoginFail {
		t.Fatal("expected dashboard_login_failure audit for bad password")
	}
}

func hasAudit(t *testing.T, pool *pgxpool.Pool, tenantIDStr, eventType string) bool {
	t.Helper()
	tid, err := uuid.Parse(tenantIDStr)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	err = pool.QueryRow(context.Background(), `
		SELECT count(*) FROM audit_events
		WHERE tenant_id = $1 AND event_type = $2
	`, tid, eventType).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func invalidTokenAuditCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM audit_events
		WHERE event_type = 'dashboard_login_failure' AND metadata->>'reason' = 'invalid_token'
	`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func loginFailureAuditCount(t *testing.T, pool *pgxpool.Pool, tenantIDStr string) int {
	t.Helper()
	tid := mustParseUUID(t, tenantIDStr)
	var n int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM audit_events
		WHERE tenant_id = $1 AND event_type = 'dashboard_login_failure'
	`, tid).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
