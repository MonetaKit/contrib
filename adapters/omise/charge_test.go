package omise

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/monetakit/monetakit/adapterkit"
)

// TestGatewayConformance replays vectors/gateway.json: canned charge responses
// -> the neutral ChargeResult classification, plus the pinned (form-encoded)
// request shape. A future TS twin must replay the same file.
func TestGatewayConformance(t *testing.T) {
	b, err := os.ReadFile("vectors/gateway.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vectors []struct {
			Name              string `json:"name"`
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
				Error         bool              `json:"error"`
				NoChargeCall  bool              `json:"noChargeCall"`
				ChargeRequest map[string]string `json:"chargeRequest"`
			} `json:"expect"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}

	for _, v := range doc.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			var calls int
			var gotPath, gotAuthUser, gotIdempotency string
			gotForm := map[string]string{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				resp, ok := v.Responses[r.URL.Path]
				if !ok {
					t.Errorf("unexpected call to %s", r.URL.Path)
					w.WriteHeader(500)
					return
				}
				gotPath = r.URL.Path
				gotAuthUser, _, _ = r.BasicAuth()
				gotIdempotency = r.Header.Get("Idempotency-Key")
				_ = r.ParseForm()
				for k := range r.PostForm {
					gotForm[k] = r.PostForm.Get(k)
				}
				w.WriteHeader(resp.Status)
				_, _ = w.Write(resp.Body)
			}))
			defer srv.Close()

			a := New(WithBaseURL(srv.URL), WithCredentials("skey_test_key"))

			res, err := a.Charge("", adapterkit.Money{Amount: v.Amount, Currency: v.Currency}, adapterkit.ChargeOpts{
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
				var got string
				switch key {
				case "path":
					got = gotPath
				case "idempotencyKey":
					got = gotIdempotency
				default:
					got = gotForm[key]
				}
				if got != want {
					t.Errorf("chargeRequest[%s] = %q, want %q", key, got, want)
				}
			}
			// Basic-auth username is the secret key, password empty.
			if calls > 0 && gotAuthUser != "skey_test_key" {
				t.Errorf("basic-auth user = %q, want the secret key", gotAuthUser)
			}
		})
	}
}
