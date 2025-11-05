package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/payflow/payflow-app/internal/auth"
	"github.com/payflow/payflow-app/internal/httpapi"
	"github.com/payflow/payflow-app/internal/payment"
	"github.com/payflow/payflow-app/internal/queue"
	"github.com/payflow/payflow-app/internal/refund"
	"github.com/payflow/payflow-app/internal/tenant"
)

// tenantResp is returned by POST /v1/tenants.
type tenantResp struct {
	TenantID string `json:"tenant_id"`
	APIKey   string `json:"api_key"`
}

func resetPayflowTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		DROP TABLE IF EXISTS async_outbox CASCADE;
		DROP TABLE IF EXISTS webhook_deliveries CASCADE;
		DROP TABLE IF EXISTS refunds CASCADE;
		DROP TABLE IF EXISTS ledger_events CASCADE;
		DROP TABLE IF EXISTS payments CASCADE;
		DROP TABLE IF EXISTS audit_events CASCADE;
		DROP TABLE IF EXISTS dashboard_users CASCADE;
		DROP TABLE IF EXISTS api_keys CASCADE;
		DROP TABLE IF EXISTS tenants CASCADE;
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func testHTTPServer(t *testing.T, pool *pgxpool.Pool, pub queue.Publisher) *httptest.Server {
	t.Helper()
	srv := &httpapi.Server{
		Pool:      pool,
		Tenants:   &tenant.Service{Pool: pool},
		Payments:  &payment.Service{Pool: pool, Q: pub},
		Refunds:   &refund.Service{Pool: pool, Q: pub},
		Pub:       pub,
		JWTSecret: []byte("integration-test-jwt-secret"),
	}
	return httptest.NewServer(httpapi.NewRouter(srv))
}

func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustCreateTenant(t *testing.T, baseURL, name string) tenantResp {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"name": name})
	res, err := http.Post(baseURL+"/v1/tenants", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("create tenant: %d %s", res.StatusCode, raw)
	}
	var tr tenantResp
	if err := json.NewDecoder(res.Body).Decode(&tr); err != nil {
		t.Fatal(err)
	}
	if tr.TenantID == "" || tr.APIKey == "" {
		t.Fatal("missing tenant_id or api_key")
	}
	return tr
}

func mustGETMe(t *testing.T, baseURL, apiKey string) map[string]any {
	t.Helper()
	m := getMe(t, baseURL, apiKey)
	if m == nil {
		t.Fatal("expected /tenants/me body")
	}
	return m
}

func mustCreatePayment(t *testing.T, baseURL, apiKey, idem string, amount int64, cur string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"amount_cents": amount, "currency": cur})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/payments", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idem)
	req.Header.Set(auth.APIKeyHeader, apiKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("create payment: %d %s", res.StatusCode, raw)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatal("missing payment id")
	}
	return id
}

func lastWebhookDeliveryID(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT id FROM webhook_deliveries WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 1
	`, tenantID).Scan(&id)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func mustPatchWebhook(t *testing.T, baseURL, apiKey, targetURL, secret string) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"url": targetURL, "signing_secret": secret})
	req, err := http.NewRequest(http.MethodPatch, baseURL+"/v1/tenants/me/webhook", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.APIKeyHeader, apiKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("patch webhook: %d %s", res.StatusCode, raw)
	}
}

func getMe(t *testing.T, baseURL, apiKey string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/tenants/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(auth.APIKeyHeader, apiKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}
