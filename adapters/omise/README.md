# Omise (Opn Payments) — PromptPay

A contrib-tier MonetaKit adapter for [Omise / Opn Payments](https://docs.opn.ooo/),
scoped to its **PromptPay push rail** — the Thailand beachhead.

## What this is

PromptPay is Thailand's national QR payment scheme: bank-to-bank, proxy-
addressed, and **push** (the payer scans and pushes money — there is no vaulted
instrument to pull off-session). PromptPay itself is a rail, not a developer
API. **Omise is the acquirer** that makes it programmable: it registers as the
biller, mints a dynamic QR per charge (so each payment reconciles to one order),
and notifies you by webhook. This adapter drives that flow.

| Capability | Support |
|---|---|
| Charge mode | `push` (PromptPay QR) |
| Currency | THB only (amounts are int64 **satang**, 1 THB = 100 satang) |
| Recurring | self-managed engine only (`recurringEngine: none`) |
| Catalog | none — `Read`/`Diff`/`Apply` are no-ops |
| Vault | unsupported (push rail; no instrument to store) |

### Flow

1. `Charge(_, {amount, currency:"thb"}, {idempotencyKey})` → `POST /charges`
   with `source[type]=promptpay`. Returns `Status:"pending"` +
   `Action{Type:"promptpay_qr", Value:<QR image URI>}`.
2. The payer scans with any Thai bank app; money settles over PromptPay.
3. Omise sends a `charge.complete` webhook → `ParseWebhook` normalizes it to a
   `paid` event (`paymentRef`/`amount`/`currency`).

## ⚠️ Retries are not deduplicated by Omise (but push makes this safe)

`Charge` sends `opts.IdempotencyKey` as Omise's `Idempotency-Key` header, but
the observed test account (API `2019-05-29`, 2026-07-09) did **not** dedup on
it — a repeated key minted a second charge. Because PromptPay is a **push**
rail this is *not* a double-collect: `Charge` only creates a QR, so a
lease-retry produces a second **unpaid** QR that expires; the payer scans and
pays at most one. Reconcile by the charge id in the `paid` webhook, never by
assuming Omise deduplicated. The key is still sent verbatim so dedup engages
automatically on any account version that honors it.

## ⚠️ Webhooks are unauthenticated

Omise does **not** sign webhook payloads. `ParseWebhook` therefore only
*normalizes* an event — it cannot authenticate it, and it **fails closed** if a
signature/secret is supplied. Treat every event as a hint and **confirm by API
lookup (`GET /charges/{id}`) before fulfilling anything.**

## Not production-ready yet

This is a skeleton. Before shipping:

- [x] Confirm the PromptPay **charge** wire shape — verified live (charge →
      `pending` + `source.scannable_code.image.download_uri`). The
      `charge.complete` **webhook** shape is still doc-based (no live event
      received yet); confirm when wiring a real endpoint.
- [x] Source and declare the THB charge bound — min **2000 satang (฿20)**,
      taken from Omise's own `invalid_charge` error, verified live 2026-07-09.
      No maximum is documented, so none is declared.
- [ ] Add the confirm-by-lookup step (`GET /charges/{id}`) callers should run.

## Running the live sandbox test

```bash
OMISE_LIVE_TEST=1 OMISE_SECRET_KEY=skey_test_... \
  go test ./adapters/omise -run TestLiveSandbox -v
```

Uses an Omise **test** secret key (`skey_test_…`) — no real money moves.

**Last verified against the Omise test API:** 2026-07-09 — `TestLiveSandboxPromptPay`
created charge `chrg_test_…`, `pending` with a `promptpay_qr` action.

## Credentials

| Env | Meaning |
|---|---|
| `OMISE_SECRET_KEY` | `skey_test_…` (test) / `skey_…` (live). Server-side only. |
