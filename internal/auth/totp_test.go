package auth

import (
	"testing"
	"time"
)

func TestAuthenticatorVectorAndWindow(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	const code = "287082" // RFC 6238 SHA-1 vector at t=59, six digits.
	if step, ok := MatchFactor(secret, code, time.Unix(59, 0)); !ok || step != 1 {
		t.Fatal(step, ok)
	}
	if _, ok := MatchFactor(secret, code, time.Unix(119, 0)); ok {
		t.Fatal("Accepted a code outside the permitted clock window")
	}
}
