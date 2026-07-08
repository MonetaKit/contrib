package paypay

import (
	"os"
	"testing"

	"github.com/monetakit/monetakit/adapterkit"
)

// TestLiveSandboxQR hits the real PayPay sandbox: create a QR charge, then
// exercise the duplicate->details fallback. Opt-in only:
//
//	PAYPAY_LIVE_TEST=1 PAYPAY_ENVIRONMENT=sandbox \
//	PAYPAY_API_KEY=... PAYPAY_API_SECRET=... PAYPAY_MERCHANT_ID=... \
//	go test ./adapters/paypay -run TestLiveSandbox -v
func TestLiveSandboxQR(t *testing.T) {
	if os.Getenv("PAYPAY_LIVE_TEST") != "1" {
		t.Skip("set PAYPAY_LIVE_TEST=1 (and PAYPAY_* credentials) to run against the sandbox")
	}
	a := New(WithSandbox())
	key := "monetakit-live-" + randomNonce()[:12]

	res, err := a.Charge("qr", adapterkit.Money{Amount: 100, Currency: "jpy"}, adapterkit.ChargeOpts{
		IdempotencyKey: key,
		Description:    "MonetaKit contrib adapter live test",
	})
	if err != nil {
		t.Fatalf("qr charge: %v", err)
	}
	if res.Status != "pending" || res.Action == nil || res.Action.Type != "paypay_qr" || res.Action.Value == "" {
		t.Fatalf("qr charge = %+v, want pending with paypay_qr action", res)
	}
	t.Logf("created QR: %s", res.Action.Value)

	// Same key again: /v2/codes rejects the duplicate and the adapter must
	// fall back to payment details — an unscanned QR reports pending.
	res2, err := a.Charge("qr", adapterkit.Money{Amount: 100, Currency: "jpy"}, adapterkit.ChargeOpts{IdempotencyKey: key})
	if err != nil {
		t.Fatalf("duplicate qr charge: %v", err)
	}
	if res2.Status != "pending" {
		t.Errorf("duplicate qr charge status = %q, want pending", res2.Status)
	}
}
