package tenant_test

import (
	"strings"
	"testing"

	"github.com/payflow/payflow-app/internal/tenant"
)

func TestValidateTenantName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string // empty = no error
	}{
		{"ok minimal", "ab", ""},
		{"ok trimmed", "  Valid Name  ", ""},
		{"too short", "a", "invalid"},
		{"empty", "", "invalid"},
		{"whitespace only", "   ", "invalid"},
		{"too long", strings.Repeat("x", 129), "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tenant.ValidateTenantName(tc.in)
			if tc.want == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want != "" && err == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.want != "" && err != tenant.ErrInvalidName {
				t.Fatalf("want ErrInvalidName, got %v", err)
			}
		})
	}
}
