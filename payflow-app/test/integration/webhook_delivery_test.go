package integration_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/payflow/payflow-app/internal/config"
	"github.com/payflow/payflow-app/internal/db"
	"github.com/payflow/payflow-app/internal/migrate"
	"github.com/payflow/payflow-app/internal/payment"
	"github.com/payflow/payflow-app/internal/queue"
	"github.com/payflow/payflow-app/internal/webhook"
)

func TestWebhookDeliverySignatureAndSuccess(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 for integration tests")
	}
	t.Setenv("JWT_SECRET", "integration-test-jwt-secret")

	const secret = "whsec_integration_test"

	var receivedBody string
	var receivedSig string
	var receivedTs string
	tsHook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		receivedSig = r.Header.Get("X-Payflow-Signature")
		receivedTs = r.Header.Get("X-Payflow-Timestamp")
		w.WriteHeader(http.StatusOK)
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

	tenant := mustCreateTenant(t, api.URL, "Webhook Del Tenant")
	mustPatchWebhook(t, api.URL, tenant.APIKey, tsHook.URL, secret)

	payID := mustCreatePayment(t, api.URL, tenant.APIKey, "wh-pay-1", 300, "EUR")
	pid := mustParseUUID(t, payID)
	if err := payment.SettleMock(ctx, pool, pid); err != nil {
		t.Fatal(err)
	}
	if err := webhook.EnqueuePaymentSettledIfNeeded(ctx, pool, pub, pid); err != nil {
		t.Fatal(err)
	}

	did := lastWebhookDeliveryID(t, pool, mustParseUUID(t, tenant.TenantID))
	if did == uuid.Nil {
		t.Fatal("expected webhook delivery row")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	if err := webhook.ProcessDelivery(ctx, pool, client, did, 5); err != nil {
		t.Fatal(err)
	}

	var st string
	err = pool.QueryRow(ctx, `SELECT status FROM webhook_deliveries WHERE id = $1`, did).Scan(&st)
	if err != nil {
		t.Fatal(err)
	}
	if st != "succeeded" {
		t.Fatalf("delivery status %q", st)
	}
	if receivedBody == "" {
		t.Fatal("webhook target was not called")
	}
	if receivedSig == "" || receivedTs == "" {
		t.Fatal("missing signature headers")
	}
	wantSig := webhook.SignatureHex(secret, receivedTs, receivedBody)
	if wantSig != receivedSig {
		t.Fatalf("signature mismatch")
	}
}
