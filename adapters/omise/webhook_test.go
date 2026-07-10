package omise

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// TestWebhookConformance replays vectors/webhooks.json: event body -> the
// neutral WebhookEvent. A future TS twin must replay the same file.
func TestWebhookConformance(t *testing.T) {
	b, err := os.ReadFile("vectors/webhooks.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vectors []struct {
			Name        string          `json:"name"`
			Payload     json.RawMessage `json:"payload"`
			Expect      map[string]any  `json:"expect"`
			ExpectError bool            `json:"expectError"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	a := New(WithCredentials("skey_test_x"))
	for _, v := range doc.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			ev, err := a.ParseWebhook(v.Payload, "", "")
			if v.ExpectError {
				if err == nil {
					t.Fatalf("want error, got event %+v", ev)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			// Compare through JSON so omitempty semantics match the vector.
			raw, _ := json.Marshal(ev)
			var got map[string]any
			_ = json.Unmarshal(raw, &got)
			if !reflect.DeepEqual(got, v.Expect) {
				t.Errorf("event = %v, want %v", got, v.Expect)
			}
		})
	}
}

// Omise webhooks are unsigned: supplying a signature or secret must fail
// closed, never silently skip verification.
func TestWebhookFailsClosedOnSignature(t *testing.T) {
	a := New(WithCredentials("skey_test_x"))
	payload := []byte(`{"key":"charge.complete","data":{"id":"chrg_1","status":"successful","amount":1,"currency":"thb"}}`)
	if _, err := a.ParseWebhook(payload, "sig", ""); err == nil {
		t.Error("signature supplied: want error, got nil")
	}
	if _, err := a.ParseWebhook(payload, "", "whsec"); err == nil {
		t.Error("secret supplied: want error, got nil")
	}
}
