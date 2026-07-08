package paypay

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/monetakit/monetakit/adapterkit"
)

// TestGatewayConformance replays vectors/gateway.json: canned OPA responses ->
// the neutral ChargeResult classification, plus the pinned request shape. A
// future TS twin must replay the same file.
func TestGatewayConformance(t *testing.T) {
	b, err := os.ReadFile("vectors/gateway.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vectors []struct {
			Name              string `json:"name"`
			Token             string `json:"token"`
			Amount            int64  `json:"amount"`
			Currency          string `json:"currency"`
			IdempotencyKey    string `json:"idempotencyKey"`
			ChargeDescription string `json:"chargeDescription"`
			Responses         map[string]struct {
				Status int             `json:"status"`
				Body   json.RawMessage `json:"body"`
			} `json:"responses"`
			Expect struct {
				Result *struct {
					Status      string `json:"status"`
					ProviderRef string `json:"providerRef"`
				} `json:"result"`
				Action *struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"action"`
				Error         bool           `json:"error"`
				NoChargeCall  bool           `json:"noChargeCall"`
				ChargeRequest map[string]any `json:"chargeRequest"`
			} `json:"expect"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}

	for _, v := range doc.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			var calls int
			var gotPath string
			var gotBody map[string]any
			var gotAuth, gotMerchant string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				resp, ok := v.Responses[r.URL.Path]
				if !ok {
					t.Errorf("unexpected call to %s", r.URL.Path)
					w.WriteHeader(500)
					return
				}
				if r.Method == http.MethodPost {
					gotPath = r.URL.Path
					raw, _ := io.ReadAll(r.Body)
					_ = json.Unmarshal(raw, &gotBody)
					gotAuth = r.Header.Get("Authorization")
					gotMerchant = r.Header.Get("X-ASSUME-MERCHANT")
				}
				w.WriteHeader(resp.Status)
				_, _ = w.Write(resp.Body)
			}))
			defer srv.Close()

			a := New(
				WithBaseURL(srv.URL),
				WithCredentials("test_key", "test_secret", "test_merchant"),
			)
			a.now = func() time.Time { return time.Unix(1700000000, 0) }
			a.nonce = func() string { return "fixed-nonce" }

			res, err := a.Charge(v.Token, adapterkit.Money{Amount: v.Amount, Currency: v.Currency}, adapterkit.ChargeOpts{
				IdempotencyKey: v.IdempotencyKey,
				Description:    v.ChargeDescription,
			})

			if v.Expect.NoChargeCall && calls != 0 {
				t.Errorf("expected no API call, got %d", calls)
			}
			if v.Expect.Error {
				if err == nil {
					t.Fatalf("want error, got result %+v", res)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Status != v.Expect.Result.Status || res.ProviderRef != v.Expect.Result.ProviderRef {
				t.Errorf("result = %s/%s, want %s/%s", res.Status, res.ProviderRef, v.Expect.Result.Status, v.Expect.Result.ProviderRef)
			}
			if v.Expect.Action != nil {
				if res.Action == nil {
					t.Fatalf("want action %+v, got nil", v.Expect.Action)
				}
				if res.Action.Type != v.Expect.Action.Type || res.Action.Value != v.Expect.Action.Value {
					t.Errorf("action = %+v, want %+v", res.Action, v.Expect.Action)
				}
			}
			for key, want := range v.Expect.ChargeRequest {
				var got any
				switch key {
				case "path":
					got = gotPath
				case "amount", "currency":
					if amt, ok := gotBody["amount"].(map[string]any); ok {
						got = amt[key]
					}
				default:
					got = gotBody[key]
				}
				// JSON numbers decode as float64 on both sides of the compare.
				if wf, ok := want.(float64); ok {
					if gf, ok := got.(float64); !ok || gf != wf {
						t.Errorf("chargeRequest[%s] = %v, want %v", key, got, want)
					}
					continue
				}
				if got != want {
					t.Errorf("chargeRequest[%s] = %v, want %v", key, got, want)
				}
			}
			if calls > 0 {
				if !strings.HasPrefix(gotAuth, "hmac OPA-Auth:test_key:") {
					t.Errorf("Authorization header not OPA-Auth signed: %q", gotAuth)
				}
				if gotMerchant != "test_merchant" {
					t.Errorf("X-ASSUME-MERCHANT = %q", gotMerchant)
				}
			}
		})
	}
}
