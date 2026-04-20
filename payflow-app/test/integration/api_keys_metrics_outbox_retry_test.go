package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/payflow/payflow-app/internal/auth"
	"github.com/payflow/payflow-app/internal/config"
	"github.com/payflow/payflow-app/internal/db"
	"github.com/payflow/payflow-app/internal/migrate"
	"github.com/payflow/payflow-app/internal/payment"
	"github.com/payflow/payflow-app/internal/queue"
	"github.com/payflow/payflow-app/internal/webhook"
)

func TestOutboxRowAfterPaymentCreate(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 for integration tests")
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
			t.Fatalf("migrate: %v", err)
		}
		t.Fatal(err)
	}

	pub := queue.NoOpPublisher{}
	api := testHTTPServer(t, pool, pub)
	t.Cleanup(api.Close)

	tenant := mustCreateTenant(t, api.URL, "Outbox Co")
	payID := mustCreatePayment(t, api.URL, tenant.APIKey, "outbox-ob-1", 50, "USD")

	var n int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM async_outbox
		WHERE kind = 'payment_settlement' AND payload->>'payment_id' = $1 AND processed_at IS NULL
	`, payID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 pending outbox row, got %d", n)
	}
}

func TestAPIKeysListAndRevoke(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 for integration tests")
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
			t.Fatalf("migrate: %v", err)
		}
		t.Fatal(err)
	}

	pub := queue.NoOpPublisher{}
	api := testHTTPServer(t, pool, pub)
	t.Cleanup(api.Close)

	tenant := mustCreateTenant(t, api.URL, "Keys Co")
	req, _ := http.NewRequest(http.MethodGet, api.URL+"/v1/tenants/me/api-keys", nil)
	req.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("list keys: %d %s", res.StatusCode, raw)
	}
	var list struct {
		Keys []struct {
			ID        string `json:"id"`
			KeyPrefix string `json:"key_prefix"`
		} `json:"api_keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Keys) < 1 {
		t.Fatal("expected at least one api key")
	}
	keyID := list.Keys[0].ID

	reqD, _ := http.NewRequest(http.MethodDelete, api.URL+"/v1/tenants/me/api-keys/"+keyID, nil)
	reqD.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	resD, err := http.DefaultClient.Do(reqD)
	if err != nil {
		t.Fatal(err)
	}
	defer resD.Body.Close()
	if resD.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resD.Body)
		t.Fatalf("revoke: %d %s", resD.StatusCode, raw)
	}

	reqMe, _ := http.NewRequest(http.MethodGet, api.URL+"/v1/tenants/me", nil)
	reqMe.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	resMe, err := http.DefaultClient.Do(reqMe)
	if err != nil {
		t.Fatal(err)
	}
	defer resMe.Body.Close()
	if resMe.StatusCode != http.StatusUnauthorized {
		t.Fatalf("after revoke, /me want 401 got %d", resMe.StatusCode)
	}
}

func TestMetricsExposesPayflowCounter(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 for integration tests")
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
			t.Fatalf("migrate: %v", err)
		}
		t.Fatal(err)
	}

	pub := queue.NoOpPublisher{}
	api := testHTTPServer(t, pool, pub)
	t.Cleanup(api.Close)

	tenant := mustCreateTenant(t, api.URL, "Metrics Co")
	_ = mustCreatePayment(t, api.URL, tenant.APIKey, "metrics-pay-1", 10, "USD")

	res, err := http.Get(api.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("metrics: %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("payflow_payments_created_total")) {
		snippet := string(body)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		t.Fatalf("metrics body missing payflow_payments_created_total: %s", snippet)
	}
}

func TestWebhookDLQRetry(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 for integration tests")
	}
	t.Setenv("JWT_SECRET", "integration-test-jwt-secret")

	tsHook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsHook.Close()

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
			t.Fatalf("migrate: %v", err)
		}
		t.Fatal(err)
	}

	pub := queue.NoOpPublisher{}
	api := testHTTPServer(t, pool, pub)
	t.Cleanup(api.Close)

	tenant := mustCreateTenant(t, api.URL, "Retry Tenant")
	mustPatchWebhook(t, api.URL, tenant.APIKey, tsHook.URL, "secret_retry")

	payID := mustCreatePayment(t, api.URL, tenant.APIKey, "retry-pay-1", 100, "EUR")
	pid := mustParseUUID(t, payID)
	if err := payment.SettleMock(ctx, pool, pid); err != nil {
		t.Fatal(err)
	}
	if err := webhook.EnqueuePaymentSettledIfNeeded(ctx, pool, pub, pid); err != nil {
		t.Fatal(err)
	}
	did := lastWebhookDeliveryID(t, pool, mustParseUUID(t, tenant.TenantID))
	if did == uuid.Nil {
		t.Fatal("expected delivery")
	}
	_, _ = pool.Exec(ctx, `UPDATE webhook_deliveries SET max_attempts = 2 WHERE id = $1`, did)

	client := &http.Client{Timeout: 3 * time.Second}
	if err := webhook.ProcessDelivery(ctx, pool, client, did, 2); err != nil {
		t.Fatal(err)
	}

	reqR, _ := http.NewRequest(http.MethodPost, api.URL+"/v1/webhook-deliveries/"+did.String()+"/retry", nil)
	reqR.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	resR, err := http.DefaultClient.Do(reqR)
	if err != nil {
		t.Fatal(err)
	}
	defer resR.Body.Close()
	if resR.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resR.Body)
		t.Fatalf("retry: %d %s", resR.StatusCode, raw)
	}

	var st string
	_ = pool.QueryRow(ctx, `SELECT status FROM webhook_deliveries WHERE id = $1`, did).Scan(&st)
	if st != "pending" {
		t.Fatalf("after retry want status pending got %q", st)
	}
}
