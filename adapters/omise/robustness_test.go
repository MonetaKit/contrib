package omise

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/monetakit/monetakit/adapterkit"
)

// The declarative gateway.json harness can only express HTTP status + body — it
// cannot drop a connection or time out. But a genuine transport failure is the
// exact case where "error vs. result" matters most: it must reach the
// scheduler's lease-retry path and NEVER consume a dunning attempt. These live
// as Go tests.

// Server unreachable (connection refused) -> error, empty result.
func TestChargeTransportFailureIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // closed before the call

	a := New(WithBaseURL(srv.URL), WithCredentials("skey_test_x"))
	res, err := a.Charge("", adapterkit.Money{Amount: 10000, Currency: "thb"}, adapterkit.ChargeOpts{IdempotencyKey: "k"})
	if err == nil {
		t.Fatalf("transport failure: want error, got result %+v", res)
	}
	if res.Status != "" || res.ProviderRef != "" {
		t.Errorf("transport failure must yield an empty result, got %+v", res)
	}
}

// Peer hangup mid-request (no response written) -> error, not a decline.
func TestChargeConnectionDropIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler) // abort the connection without a response
	}))
	defer srv.Close()

	a := New(WithBaseURL(srv.URL), WithCredentials("skey_test_x"))
	if _, err := a.Charge("", adapterkit.Money{Amount: 10000, Currency: "thb"}, adapterkit.ChargeOpts{IdempotencyKey: "k"}); err == nil {
		t.Fatal("connection drop: want error, got nil")
	}
}

// A retried charge sends the colon-form engine key VERBATIM on every call and
// classifies deterministically. (Note: the observed Omise test account, API
// 2019-05-29, does NOT dedup on the Idempotency-Key — see Charge's doc — so
// this pins the adapter's own behavior, not a server-side replay: the key must
// never be mangled between attempts, since a sanitizer would both defeat any
// version that DOES dedup and obscure reconciliation.) Two calls against one
// server; not expressible in the single-call gateway harness.
func TestChargeSendsIdempotencyKeyVerbatimOnRetry(t *testing.T) {
	const body = `{"object":"charge","id":"chrg_replay","status":"pending","amount":10000,"currency":"thb","source":{"object":"source","type":"promptpay","scannable_code":{"type":"qr","image":{"download_uri":"https://api.omise.co/q"}}}}`
	var sentKeys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentKeys = append(sentKeys, r.Header.Get("Idempotency-Key"))
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	a := New(WithBaseURL(srv.URL), WithCredentials("skey_test_x"))
	const key = "sub_1:1700000000:0"
	call := func() adapterkit.ChargeResult {
		res, err := a.Charge("", adapterkit.Money{Amount: 10000, Currency: "thb"}, adapterkit.ChargeOpts{IdempotencyKey: key})
		if err != nil {
			t.Fatalf("charge: %v", err)
		}
		return res
	}
	first, second := call(), call()

	if first.Status != second.Status || first.ProviderRef != second.ProviderRef {
		t.Errorf("replay diverged: %+v vs %+v", first, second)
	}
	if first.Action == nil || second.Action == nil || first.Action.Value != second.Action.Value {
		t.Errorf("replay QR diverged: %+v vs %+v", first.Action, second.Action)
	}
	for i, k := range sentKeys {
		if k != key {
			t.Errorf("call %d sent Idempotency-Key %q, want the verbatim colon key %q", i, k, key)
		}
	}
}

// A webhook body that isn't a JSON object must error, never silently normalize
// to a zero-value event. (Vector files can only hold valid JSON values, so the
// malformed/empty/array cases live here.)
func TestParseWebhookMalformedBodyIsError(t *testing.T) {
	a := New(WithCredentials("skey_test_x"))
	for _, payload := range [][]byte{[]byte(``), []byte(`{ not json`), []byte(`[]`)} {
		if ev, err := a.ParseWebhook(payload, "", ""); err == nil {
			t.Errorf("payload %q: want error, got event %+v", payload, ev)
		}
	}
}
