package omise

import (
	"testing"

	"github.com/monetakit/monetakit/adapterkit"
	"github.com/monetakit/monetakit/adapterkit/certify"
)

// TestCertify runs the shared adapter certification battery (see
// adapterkit/certify/SCENARIOS.md in core). Omise declares push + a webhook
// parser, so the derived obligations are PSH-01 (QR Action), WHK-PAID (the
// paid triple), WHK-CLOSED (fail-closed on the unsigned rail) and CUR-REJECT
// (thb declared). The httptest server the harness spins up is driven via
// WithBaseURL — no network in CI.
func TestCertify(t *testing.T) {
	certify.Run(t, certify.Target{
		New: func(t testing.TB, baseURL string) adapterkit.PaymentProvider {
			return New(
				WithBaseURL(baseURL),
				WithCredentials("skey_test_certify"),
			)
		},
		ScenariosFile: "vectors/certify.json",
	})
}
