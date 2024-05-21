package payment_test

import (
	"testing"

	"github.com/payflow/payflow-app/internal/payment"
)

func TestRequestFingerprint_stable(t *testing.T) {
	t.Parallel()
	a := payment.RequestFingerprint(100, "eur")
	b := payment.RequestFingerprint(100, "EUR")
	if a != b || a == "" {
		t.Fatalf("fingerprint mismatch: %q vs %q", a, b)
	}
}

func TestRequestFingerprint_differsByAmount(t *testing.T) {
	t.Parallel()
	x := payment.RequestFingerprint(100, "EUR")
	y := payment.RequestFingerprint(101, "EUR")
	if x == y {
		t.Fatal("expected different fingerprints")
	}
}
