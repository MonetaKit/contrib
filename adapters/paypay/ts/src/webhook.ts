// PayPay webhook normalization — the TS twin of the Go adapter's
// ParseWebhook, replaying the same vectors/webhooks.json.
//
// ⚠️ PayPay webhooks are UNSIGNED (no signature header, no shared secret;
// PayPay's guidance is IP allowlisting). This parser only NORMALIZES — it
// cannot authenticate. Treat every event as a hint: confirm via
// GET /v2/payments/{merchantPaymentId} before fulfilling. Fail closed: a
// supplied signature/secret is an error, never a silently skipped check.

export interface WebhookEvent {
  type: "subscribed" | "updated" | "canceled" | "payment_failed" | "paid" | "ignored";
  raw: string;
  customer?: string;
  subscriptionId?: string;
  priceKey?: string;
  // One-time inbound payment (type "paid") — set together, per adapterkit.
  paymentRef?: string;
  amount?: number;
  currency?: string;
}

export function parseWebhook(payload: string | Uint8Array, signature?: string, secret?: string): WebhookEvent {
  if (signature || secret) {
    throw new Error(
      "paypay: webhooks are unsigned; a signature/secret was supplied but cannot be verified — confirm events via API lookup instead",
    );
  }
  const text = typeof payload === "string" ? payload : new TextDecoder().decode(payload);
  const ev = JSON.parse(text) as {
    notification_type?: string;
    state?: string;
    order_id?: string;
    merchant_order_id?: string;
    order_amount?: number;
    userAuthorizationId?: string;
  };
  switch (ev.notification_type) {
    case "Transaction": {
      const raw = `Transaction:${ev.state}`;
      switch (ev.state) {
        case "COMPLETED":
          return {
            type: "paid",
            raw,
            paymentRef: ev.order_id || ev.merchant_order_id,
            amount: ev.order_amount,
            currency: "jpy",
          };
        case "FAILED":
          return { type: "payment_failed", raw };
        default: // AUTHORIZED, CANCELED, EXPIRED, EXPIRED_USER_CONFIRMATION
          return { type: "ignored", raw };
      }
    }
    // PayPay's docs spell this event "authroization"; accept the corrected
    // spelling too in case they ever fix it.
    case "customer.authroization.revoked":
    case "customer.authorization.revoked":
      return { type: "canceled", raw: ev.notification_type, customer: ev.userAuthorizationId };
  }
  return { type: "ignored", raw: ev.notification_type ?? "" };
}
