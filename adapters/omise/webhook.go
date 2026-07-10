package omise

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monetakit/monetakit/adapterkit"
)

// ⚠️ Omise webhooks are UNSIGNED. Omise sends no signature header and shares no
// webhook secret — its guidance is to confirm authenticity by re-fetching the
// object from the API (GET /charges/{id}). This parser therefore only
// NORMALIZES the event; it cannot authenticate it. Treat every event as a hint
// and confirm by lookup before fulfilling.
//
// Fail closed: if the caller supplies a signature or secret, we refuse rather
// than silently skip a verification the rail cannot provide.
func (a *Adapter) ParseWebhook(payload []byte, signature, secret string) (adapterkit.WebhookEvent, error) {
	if signature != "" || secret != "" {
		return adapterkit.WebhookEvent{}, fmt.Errorf("omise: webhooks are unsigned; a signature/secret was supplied but cannot be verified — confirm events via GET /charges/{id} instead")
	}
	var ev struct {
		Object string `json:"object"`
		Key    string `json:"key"` // e.g. charge.complete, charge.create
		Data   struct {
			Object   string `json:"object"`
			ID       string `json:"id"`
			Status   string `json:"status"` // pending | successful | failed | expired | reversed
			Amount   int64  `json:"amount"`
			Currency string `json:"currency"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		return adapterkit.WebhookEvent{}, fmt.Errorf("omise: webhook payload: %w", err)
	}

	// charge.complete is the terminal event for a PromptPay charge — the payer
	// scanned and the push settled (or failed).
	if ev.Key == "charge.complete" {
		switch ev.Data.Status {
		case "successful":
			// Never emit a partial paid event: adapterkit's invariant is that
			// paymentRef/amount/currency travel together.
			if ev.Data.ID == "" || ev.Data.Amount <= 0 || ev.Data.Currency == "" {
				return adapterkit.WebhookEvent{}, fmt.Errorf("omise: charge.complete successful without id/amount/currency — refusing to emit a partial paid event")
			}
			return adapterkit.WebhookEvent{
				Type:       "paid",
				Raw:        ev.Key,
				PaymentRef: ev.Data.ID,
				Amount:     ev.Data.Amount,
				Currency:   strings.ToLower(ev.Data.Currency),
			}, nil
		case "failed", "expired", "reversed":
			// reversed is a terminal non-success the same way Charge() treats
			// it as a decline — never let it fall through to "ignored".
			return adapterkit.WebhookEvent{Type: "payment_failed", Raw: ev.Key}, nil
		}
	}

	// Every other event (charge.create, refund.*, etc.) normalizes to ignored,
	// never an error.
	raw := ev.Key
	if raw == "" {
		raw = ev.Object
	}
	return adapterkit.WebhookEvent{Type: "ignored", Raw: raw}, nil
}

var _ adapterkit.WebhookParser = (*Adapter)(nil)
