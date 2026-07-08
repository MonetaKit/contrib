package paypay

import (
	"encoding/json"
	"testing"
)

// Golden values generated with PayPay's official node SDK auth code
// (crypto-js MD5/HMAC-SHA256, fixed nonce + epoch) — the Go port must match
// byte for byte or real requests will 401.
func TestAuthHeaderGolden(t *testing.T) {
	body := continuousChargeRequest{
		MerchantPaymentID:   "sub_1-1700000000-0",
		UserAuthorizationID: "ua_1",
		Amount:              moneyJSON{Amount: 1200, Currency: "JPY"},
		RequestedAt:         1700000000,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	// The digest is over the exact wire bytes: Go's marshaling must match
	// node's JSON.stringify for the same field order.
	wantJSON := `{"merchantPaymentId":"sub_1-1700000000-0","userAuthorizationId":"ua_1","amount":{"amount":1200,"currency":"JPY"},"requestedAt":1700000000}`
	if string(raw) != wantJSON {
		t.Fatalf("marshaled body drifted from the signed golden bytes:\n got %q\nwant %q", raw, wantJSON)
	}

	got := authHeader("test_client", "test_secret", "POST", "/v1/subscription/payments", raw, "fixed-nonce", 1700000000)
	want := "hmac OPA-Auth:test_client:zMTNgAd6+TmrBk5R7MFnktNZvqAyPK7wRrs463lL5Nw=:fixed-nonce:1700000000:Uk/Q2Rb9c83A5fGGq/p+pA=="
	if got != want {
		t.Errorf("POST header:\n got %q\nwant %q", got, want)
	}

	got = authHeader("test_client", "test_secret", "GET", "/v2/payments/sub_1-1700000000-0", nil, "fixed-nonce", 1700000000)
	want = "hmac OPA-Auth:test_client:APmTrlUINlSWb9ZwQ9wYmaGp+fsajNIZH9BJ+1AO+MY=:fixed-nonce:1700000000:empty"
	if got != want {
		t.Errorf("GET header:\n got %q\nwant %q", got, want)
	}
}
