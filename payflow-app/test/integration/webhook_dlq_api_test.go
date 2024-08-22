package integration_test

import (
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

func TestWebhookDLQListAndDetailAPI(t *testing.T) {
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

	tenant := mustCreateTenant(t, api.URL, "DLQ Tenant")
	mustPatchWebhook(t, api.URL, tenant.APIKey, tsHook.URL, "secret_dlq")

	payID := mustCreatePayment(t, api.URL, tenant.APIKey, "dlq-pay-1", 100, "EUR")
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

	var st string
	_ = pool.QueryRow(ctx, `SELECT status FROM webhook_deliveries WHERE id = $1`, did).Scan(&st)
	if st != "dlq" {
		t.Fatalf("want dlq got %q", st)
	}

	req, _ := http.NewRequest(http.MethodGet, api.URL+"/v1/webhook-deliveries?status=dlq", nil)
	req.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("list dlq %d %s", res.StatusCode, raw)
	}
	var listOut struct {
		Deliveries []map[string]any `json:"deliveries"`
	}
	if err := json.NewDecoder(res.Body).Decode(&listOut); err != nil {
		t.Fatal(err)
	}
	if len(listOut.Deliveries) < 1 {
		t.Fatal("expected at least one dlq row in list")
	}

	req2, _ := http.NewRequest(http.MethodGet, api.URL+"/v1/webhook-deliveries/"+did.String(), nil)
	req2.Header.Set(auth.APIKeyHeader, tenant.APIKey)
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(res2.Body)
		t.Fatalf("get detail %d %s", res2.StatusCode, raw)
	}
	var detail map[string]any
	if err := json.NewDecoder(res2.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail["status"].(string) != "dlq" {
		t.Fatalf("detail status %v", detail["status"])
	}
}
