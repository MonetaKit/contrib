package paypay

import (
	"testing"

	"github.com/monetakit/monetakit/adapterkit"
	"github.com/monetakit/monetakit/adapterkit/certify"
)

// TestCertify runs the shared adapter certification battery (see
// adapterkit/certify/SCENARIOS.md in core). PayPay declares both charge
// modes, so this is the first adapter exercising the full push+pull battery:
// PSH-01 (QR Action), WHK-PAID (the paid triple), CUR-REJECT (jpy declared),
// and the WHK-CLOSED fail-closed law for an unsigned rail.
func TestCertify(t *testing.T) {
	certify.Run(t, certify.Target{
		New: func(t testing.TB, baseURL string) adapterkit.PaymentProvider {
			return New(
				WithBaseURL(baseURL),
				WithCredentials("test_key", "test_secret", "test_merchant"),
			)
		},
		ScenariosFile: "vectors/certify.json",
	})
}
