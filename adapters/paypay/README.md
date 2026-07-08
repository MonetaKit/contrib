# PayPay (Japan) — contrib adapter

[PayPay](https://developer.paypay.ne.jp/) via the Open Payment API (OPA).
JPY only. A hybrid rail, and this adapter implements both sides:

| Flow | `Charge` token | What happens |
|---|---|---|
| Off-session pull | a `userAuthorizationId` (from `Vault`) | `POST /v1/subscription/payments` — the self-managed engine's billing path |
| QR push | the literal `"qr"` | `POST /v2/codes` → `pending` + payment URL in `ChargeResult.Action{Type:"paypay_qr"}` |

`Vault` accepts the account-link `responseToken` JWT (HS256, keyed with the
Base64-decoded API secret), verifies it (alg pinned, constant-time MAC,
iss/aud/exp/result), and returns the embedded `userAuthorizationId`.

`ChargeOpts.IdempotencyKey` becomes `merchantPaymentId`. PayPay replays
duplicates on the continuous-payment endpoint; on `/v2/codes` it rejects them,
so a retried QR charge falls back to a payment-details lookup. Engine keys
(`sub:period:attempt`) contain colons PayPay forbids — they map
deterministically to `sanitized[:40]-sha256[:16]`.

## ⚠️ Not production-ready: webhooks are unsigned

PayPay webhooks carry **no signature** (PayPay's guidance is IP allowlisting).
`ParseWebhook` only normalizes — it cannot authenticate, and it fails closed
if you pass a signature/secret. Treat every event as a hint and **confirm via
`GET /v2/payments/{merchantPaymentId}` before fulfilling**. Preauth
(`isAuthorization`), refunds, and the account-link session API are not yet
implemented.

## Setup

Credentials from the [developer portal](https://developer.paypay.ne.jp/):

```
PAYPAY_API_KEY / PAYPAY_API_SECRET / PAYPAY_MERCHANT_ID
PAYPAY_ENVIRONMENT=sandbox        # default is production
```

Sandbox test users pay by scanning with the sandbox PayPay app (sign-in
screen → tap the header 7× → Developer Mode, OTP `1234`).

## Tests

- `vectors/gateway.json`, `vectors/webhooks.json` — language-neutral
  conformance vectors, replayed by BOTH implementations: the Go tests and
  the TS twin in `ts/` (`@monetakit/paypay`, MSW-mocked) — the two cannot
  drift on charge classification, request shape, or webhook normalization.
- `auth_test.go` — golden signatures generated with PayPay's official node
  SDK; the Go port must match byte for byte.
- `live_test.go` — opt-in sandbox round-trip: `PAYPAY_LIVE_TEST=1` + the
  env vars above (verified against the real sandbox 2026-07-08).
