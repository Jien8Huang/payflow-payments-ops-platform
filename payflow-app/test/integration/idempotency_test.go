package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/payflow/payflow-app/internal/auth"
	"github.com/payflow/payflow-app/internal/config"
	"github.com/payflow/payflow-app/internal/db"
	"github.com/payflow/payflow-app/internal/migrate"
	"github.com/payflow/payflow-app/internal/payment"
	"github.com/payflow/payflow-app/internal/queue"
)

func TestConcurrentPaymentIdempotency(t *testing.T) {
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

	tenant := mustCreateTenant(t, ts.URL, "Idem Tenant")
	const idemKey = "idem-concurrent-001"
	const n = 25
	body := []byte(`{"amount_cents":500,"currency":"EUR"}`)

	start := make(chan struct{})
	var wg sync.WaitGroup
	statusCodes := make(chan int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/payments", bytes.NewReader(body))
			if err != nil {
				statusCodes <- 0
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", idemKey)
			req.Header.Set(auth.APIKeyHeader, tenant.APIKey)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				statusCodes <- 0
				return
			}
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
			statusCodes <- res.StatusCode
		}()
	}
	close(start)
	wg.Wait()
	close(statusCodes)
	for c := range statusCodes {
		if c != http.StatusCreated && c != http.StatusOK {
			t.Fatalf("unexpected status %d", c)
		}
	}

	var count int
	tid := mustParseUUID(t, tenant.TenantID)
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM payments
		WHERE tenant_id = $1 AND idempotency_key = $2
	`, tid, idemKey).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 payment row, got %d", count)
	}
}

func TestIdempotencyKeyBodyMismatch409(t *testing.T) {
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
	tenant := mustCreateTenant(t, ts.URL, "Mismatch Tenant")

	post := func(body []byte) int {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/payments", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "same-key-mismatch")
		req.Header.Set(auth.APIKeyHeader, tenant.APIKey)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if got := post([]byte(`{"amount_cents":100,"currency":"EUR"}`)); got != http.StatusCreated {
		t.Fatalf("first post %d", got)
	}
	if got := post([]byte(`{"amount_cents":200,"currency":"EUR"}`)); got != http.StatusConflict {
		t.Fatalf("second post want 409 got %d", got)
	}
}

func TestDoubleSettleWorkerIdempotency(t *testing.T) {
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
	tenant := mustCreateTenant(t, ts.URL, "Settle Tenant")

	body := []byte(`{"amount_cents":999,"currency":"usd"}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "settle-dedupe-1")
	req.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("create payment %d %s", res.StatusCode, raw)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	pid, err := uuid.Parse(out["id"].(string))
	if err != nil {
		t.Fatal(err)
	}

	if err := payment.SettleMock(ctx, pool, pid); err != nil {
		t.Fatal(err)
	}
	if err := payment.SettleMock(ctx, pool, pid); err != nil {
		t.Fatal(err)
	}

	var n int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM ledger_events
		WHERE payment_id = $1 AND dedupe_key = $2
	`, pid, payment.LedgerSettlementCompleted).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 settlement ledger row, got %d", n)
	}
	var st string
	err = pool.QueryRow(ctx, `SELECT status FROM payments WHERE id = $1`, pid).Scan(&st)
	if err != nil {
		t.Fatal(err)
	}
	if st != "succeeded" {
		t.Fatalf("status want succeeded got %q", st)
	}
}
