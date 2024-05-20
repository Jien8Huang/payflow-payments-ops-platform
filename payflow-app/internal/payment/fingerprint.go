package payment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

type fingerprintBody struct {
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

// RequestFingerprint is a stable SHA-256 hex digest of the idempotent payment intent (R4).
func RequestFingerprint(amountCents int64, currency string) string {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	b, err := json.Marshal(fingerprintBody{AmountCents: amountCents, Currency: cur})
	if err != nil {
		// struct-only marshal does not fail
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
