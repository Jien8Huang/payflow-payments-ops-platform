package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/payflow/payflow-app/internal/auth"
)

func TestHashAPIKey_stable(t *testing.T) {
	t.Parallel()
	raw := "pf_live_" + strings.Repeat("ab", 16)
	h1 := auth.HashAPIKey(raw)
	h2 := auth.HashAPIKey(raw)
	if h1 != h2 {
		t.Fatal("hash not stable")
	}
	if len(h1) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(h1))
	}
}

func TestGenerateAPIKey_prefix(t *testing.T) {
	t.Parallel()
	k, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(k, "pf_live_") {
		t.Fatalf("unexpected prefix: %q", k)
	}
}

func TestDashboardJWT_roundTrip(t *testing.T) {
	t.Parallel()
	secret := []byte("unit-test-secret")
	tid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	uid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tok, err := auth.SignDashboardToken(secret, tid, uid, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	gotTid, gotUid, err := auth.ParseDashboardBearer(secret, "Bearer "+tok)
	if err != nil {
		t.Fatal(err)
	}
	if gotTid != tid || gotUid != uid {
		t.Fatalf("got %s %s want %s %s", gotTid, gotUid, tid, uid)
	}
}

func TestParseDashboardBearer_rejectsGarbage(t *testing.T) {
	t.Parallel()
	secret := []byte("unit-test-secret")
	_, _, err := auth.ParseDashboardBearer(secret, "Bearer not-a-jwt")
	if err == nil {
		t.Fatal("expected error")
	}
}
