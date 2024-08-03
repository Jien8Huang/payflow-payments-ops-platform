package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/payflow/payflow-app/internal/auth"
	"github.com/payflow/payflow-app/internal/config"
	"github.com/payflow/payflow-app/internal/db"
	"github.com/payflow/payflow-app/internal/migrate"
	"github.com/payflow/payflow-app/internal/queue"
)

func TestTenantIsolationPaymentGET(t *testing.T) {
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

	ts := testHTTPServer(t, pool, queue.NoOpPublisher{})
	t.Cleanup(ts.Close)

	a := mustCreateTenant(t, ts.URL, "Pay Iso A")
	b := mustCreateTenant(t, ts.URL, "Pay Iso B")

	body := []byte(`{"amount_cents":50,"currency":"EUR"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "iso-pay-1")
	req.Header.Set(auth.APIKeyHeader, a.APIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("create %d %s", res.StatusCode, raw)
	}
	var created map[string]any
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	pid := created["id"].(string)

	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/payments/"+pid, nil)
	req2.Header.Set(auth.APIKeyHeader, b.APIKey)
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant GET want 404 got %d", res2.StatusCode)
	}
}
