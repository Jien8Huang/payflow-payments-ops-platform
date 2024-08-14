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
	"github.com/payflow/payflow-app/internal/payment"
	"github.com/payflow/payflow-app/internal/queue"
)

func TestRefundRejectedWhenPaymentPending(t *testing.T) {
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
	tenant := mustCreateTenant(t, ts.URL, "Refund Pending Tenant")

	body := []byte(`{"amount_cents":100,"currency":"EUR"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "pay-refund-pending-1")
	req.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("payment %d %s", res.StatusCode, raw)
	}
	var pay map[string]any
	if err := json.NewDecoder(res.Body).Decode(&pay); err != nil {
		t.Fatal(err)
	}
	pid := pay["id"].(string)

	refBody := []byte(`{"amount_cents":0}`)
	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/payments/"+pid+"/refunds", bytes.NewReader(refBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "refund-while-pending")
	req2.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		raw, _ := io.ReadAll(res2.Body)
		t.Fatalf("refund on pending payment: want 409 got %d %s", res2.StatusCode, raw)
	}
}

func TestRefundHappyPathAfterSettlement(t *testing.T) {
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
	tenant := mustCreateTenant(t, ts.URL, "Refund OK Tenant")

	payBody := []byte(`{"amount_cents":250,"currency":"EUR"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/payments", bytes.NewReader(payBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "pay-for-refund-ok")
	req.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var pay map[string]any
	_ = json.NewDecoder(res.Body).Decode(&pay)
	pid := mustParseUUID(t, pay["id"].(string))

	if err := payment.SettleMock(ctx, pool, pid); err != nil {
		t.Fatal(err)
	}

	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/payments/"+pid.String()+"/refunds", http.NoBody)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "refund-full-1")
	req2.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(res2.Body)
		t.Fatalf("refund %d %s", res2.StatusCode, raw)
	}
	var ref map[string]any
	if err := json.NewDecoder(res2.Body).Decode(&ref); err != nil {
		t.Fatal(err)
	}
	if ref["status"].(string) != "pending" {
		t.Fatalf("status %v", ref["status"])
	}
}
