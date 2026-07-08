# Contributing

## Ground rules

- One adapter/format per PR.
- Every PR adds an entry under `[Unreleased]` in [CHANGELOG.md](CHANGELOG.md)
  ([Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format).
- This code touches money and payment credentials. Reviews are strict on
  idempotency, error handling, and secret handling — that's the point of
  hosting contrib in the org instead of an awesome-list.
- Never roll your own crypto: webhook signature verification must use the
  PSP's official SDK/verification scheme.
- Each contribution has a `MAINTAINERS` entry (GitHub handle). Unmaintained
  contributions get archived, not shipped.

## Adding a PSP adapter (`adapters/<psp>/`)

1. Copy `adapters/example/` to `adapters/<psp>/` (lowercase, one word:
   `tappay`, `newebpay`, `adyen`).
2. Fill in `capability.json` **honestly** — `moneta plan` uses it to fail
   loudly at plan-time instead of mid-apply. Mirror the fields in
   `adapterkit.Capabilities` (see `adapters/stripe/capability.json` in core
   for a filled-in reference).
3. Implement `adapterkit.PaymentProvider`. Optional capabilities
   (`UsageReporter`, `WebhookParser`, `DriftDetector`, `SubscriptionCounter`)
   are separate interfaces — implement what the PSP supports, skip the rest.
4. Idempotency is non-negotiable: `Charge` must honor
   `ChargeOpts.IdempotencyKey`; `Apply` must be safe to re-run.
5. Tests:
   - Run the shared conformance vectors from core's `conformance/` tree
     (gateway + webhooks at minimum; see
     `adapters/stripe/stripe_gateway_conformance_test.go` in core for the
     pattern).
   - Record real API interactions with VCR cassettes (`go-vcr`) so CI runs
     without live credentials. **Scrub secrets from cassettes.**
6. `make check` must pass.

## Adding an invoicing or tax format (`invoicing/<code>/`, `tax/<code>/`)

Directory name: lowercase country/system code (`tw-einvoice`,
`jp-qualified-invoice`, `eu-peppol`).

These are **data-first** — the goal is that a domain expert can contribute
without writing engine code:

1. `README.md` — what the format is, links to the official spec (with
   version/date), scope of what's covered.
2. `format.json` (or the official XSD/JSON Schema) — the target format's
   structure and field constraints.
3. `mapping.json` — how MonetaKit terms (charge, customer, price, tax fields)
   map onto the format's fields.
4. `vectors/` — golden test vectors: input (MonetaKit canonical JSON) →
   expected output document. These are the contract; the Go/TS execution
   layer lives in core and is validated against your vectors.

The exact schema for `mapping.json` is still settling — for now, open an
issue first and we'll shape it together. Early contributions here define the
convention.

## Dev setup

```bash
# sibling checkouts (go.mod replace expects ../monetakit)
git clone git@github.com:MonetaKit/monetakit.git
git clone git@github.com:MonetaKit/contrib.git
cd contrib
make check   # fmt + vet + build + test
```
