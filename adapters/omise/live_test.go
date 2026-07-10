package omise

import (
	"os"
	"testing"

	"github.com/monetakit/monetakit/adapterkit"
)

// TestLiveSandboxPromptPay hits the real Omise test API: create a PromptPay
// charge and assert it comes back pending with a scannable QR. Opt-in only
// (skipped in CI):
//
//	OMISE_LIVE_TEST=1 OMISE_SECRET_KEY=skey_test_... \
//	go test ./adapters/omise -run TestLiveSandbox -v
//
// Uses an Omise TEST secret key (skey_test_…) — no real money moves.
func TestLiveSandboxPromptPay(t *testing.T) {
	if os.Getenv("OMISE_LIVE_TEST") != "1" {
		t.Skip("set OMISE_LIVE_TEST=1 (and OMISE_SECRET_KEY=skey_test_...) to run against Omise's test API")
	}
	if os.Getenv("OMISE_SECRET_KEY") == "" {
		t.Fatal("OMISE_SECRET_KEY is required for the live test")
	}
	a := New() // reads OMISE_SECRET_KEY from the env

	res, err := a.Charge("", adapterkit.Money{Amount: 10000, Currency: "thb"}, adapterkit.ChargeOpts{
		IdempotencyKey: "monetakit-live-" + t.Name(),
		Description:    "MonetaKit contrib adapter live test",
	})
	if err != nil {
		t.Fatalf("promptpay charge: %v", err)
	}
	if res.Status != "pending" || res.Action == nil || res.Action.Type != "promptpay_qr" || res.Action.Value == "" {
		t.Fatalf("charge = %+v, want pending with a promptpay_qr action", res)
	}
	t.Logf("created PromptPay QR: %s (charge %s)", res.Action.Value, res.ProviderRef)
}
