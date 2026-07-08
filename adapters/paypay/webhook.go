package paypay

import (
	"encoding/json"
	"fmt"

	"github.com/monetakit/monetakit/adapterkit"
)

// ⚠️ PayPay webhooks are UNSIGNED. There is no signature header and no shared
// secret — PayPay's only guidance is to allowlist their egress IPs. This
// parser therefore only NORMALIZES the event; it cannot authenticate it.
// Treat every event as a hint: confirm via GET /v2/payments/{merchantPaymentId}
// (or /v2/codes/payments/...) before fulfilling anything.
//
// Fail closed: if the caller supplies a signature or secret, we refuse rather
// than silently skip a verification the rail cannot provide.
func (a *Adapter) ParseWebhook(payload []byte, signature, secret string) (adapterkit.WebhookEvent, error) {
	if signature != "" || secret != "" {
		return adapterkit.WebhookEvent{}, fmt.Errorf("paypay: webhooks are unsigned; a signature/secret was supplied but cannot be verified — confirm events via API lookup instead")
	}
	var ev struct {
		NotificationType    string `json:"notification_type"`
		State               string `json:"state"`
		OrderID             string `json:"order_id"`
		MerchantOrderID     string `json:"merchant_order_id"`
		OrderAmount         int64  `json:"order_amount"`
		UserAuthorizationID string `json:"userAuthorizationId"` // revocation events
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		return adapterkit.WebhookEvent{}, fmt.Errorf("paypay: webhook payload: %w", err)
	}
	switch ev.NotificationType {
	case "Transaction":
		raw := "Transaction:" + ev.State
		switch ev.State {
		case "COMPLETED":
			ref := ev.OrderID
			if ref == "" {
				ref = ev.MerchantOrderID
			}
			// Never emit a partial paid event: adapterkit's invariant is that
			// paymentRef/amount/currency travel together, and an inbound
			// payment nobody can reconcile is noise at best.
			if ref == "" || ev.OrderAmount <= 0 {
				return adapterkit.WebhookEvent{}, fmt.Errorf("paypay: Transaction COMPLETED without order identifier/amount — refusing to emit a partial paid event")
			}
			return adapterkit.WebhookEvent{
				Type:       "paid",
				Raw:        raw,
				PaymentRef: ref,
				Amount:     ev.OrderAmount,
				Currency:   "jpy",
			}, nil
		case "FAILED":
			return adapterkit.WebhookEvent{Type: "payment_failed", Raw: raw}, nil
		default: // AUTHORIZED, CANCELED, EXPIRED, EXPIRED_USER_CONFIRMATION
			return adapterkit.WebhookEvent{Type: "ignored", Raw: raw}, nil
		}
	// PayPay's docs spell this event type "authroization" — accept the
	// documented spelling and the corrected one, in case they ever fix it.
	case "customer.authroization.revoked", "customer.authorization.revoked":
		return adapterkit.WebhookEvent{
			Type:     "canceled",
			Raw:      ev.NotificationType,
			Customer: ev.UserAuthorizationID,
		}, nil
	}
	return adapterkit.WebhookEvent{Type: "ignored", Raw: ev.NotificationType}, nil
}

var _ adapterkit.WebhookParser = (*Adapter)(nil)
