# Contributing

## Ground rules

- One adapter/format per PR; atomic commits (one topic each).
- Every PR adds an entry under `[Unreleased]` in [CHANGELOG.md](CHANGELOG.md)
  ([Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format).
- This code touches money and payment credentials. Reviews are strict on
  idempotency, error handling, and secret handling — that's the point of
  hosting contrib in the org instead of an awesome-list.
- **Money is int64** in the smallest currency unit, everywhere, in every
  language's wire shape. Never narrow a money type; widen the consumer.
- **Declare only what you can source.** Capability facts (charge bounds,
  currency support) need a documented origin — an undeclared bound is honest,
  an invented one is a bug. Same for README claims: date-stamp anything you
  verified against a live sandbox.
- Never roll your own crypto: webhook signature verification must use the
  PSP's official SDK/verification scheme. If the PSP's webhooks are
  **unsigned**, the parser must fail closed when a signature/secret is
  supplied, and the adapter README must say loudly that events are
  unauthenticated hints to confirm by API lookup.
- Each contribution has a `MAINTAINERS` entry (GitHub handles, covering every
  language in the directory). Unmaintained contributions get archived, not
  shipped.

## Adding a PSP adapter (`adapters/<psp>/`)

Start by copying `adapters/example/` (minimal, compiles); use
`adapters/paypay/` as the full reference — it shows every convention below in
working form.

### 1. Declare capabilities

`capability.json` + `Capabilities()` are two declarations of the same fact —
keep the drift test from the skeleton. Declare **honestly**: `moneta plan`
uses this to reject incompatible Planfiles at plan-time instead of failing
mid-apply, and the certification battery derives your test obligations from
it. Cover `chargeModes` (`pull` / `push`) and `currencies` (with min/max
charge bounds only if the PSP documents them).

### 2. Implement the contract

`adapterkit.PaymentProvider` is required. Optional capabilities
(`UsageReporter`, `WebhookParser`, `DriftDetector`, `SubscriptionCounter`)
are separate interfaces — implement what the PSP supports, skip the rest
(declaring but not implementing, or implementing without declaring, fails
certification).

Non-negotiables the battery and reviewers will hold you to:

- `Charge` honors `ChargeOpts.IdempotencyKey`; a retried period-charge must
  never double-collect. If the PSP restricts key charset/length, transform
  deterministically (see paypay's `merchantPaymentID`).
- **A business decline is a result** (`ChargeResult{Status:"failed"}`, nil
  error); **a transport failure is an error**. This split decides whether the
  billing engine consumes a dunning attempt or lease-retries — the most
  load-bearing behavior in the whole contract.
- Push flows return `pending` plus a populated `ChargeResult.Action`.
- Webhook `paid` events set `paymentRef`/`amount`/`currency` together or
  refuse to emit; unknown events normalize to `ignored`, never an error.
- Reject unsupported currencies before touching the network.
- Wrap transport-layer surprises (non-JSON bodies, malformed payloads) in
  `<psp>:`-prefixed errors with method/path/status context.

### 3. Ship vectors (`vectors/`)

Language-neutral JSON fixtures, colocated with the adapter:

- `gateway.json` — canned PSP responses → expected neutral `ChargeResult`
  (plus pinned request shape).
- `webhooks.json` — notification payloads → expected neutral `WebhookEvent`
  (`expectError` for payloads that must be refused).
- `certify.json` — scenario fixtures for the certification battery.

These are the adapter's behavior contract: the Go tests replay them, and a TS
twin must replay the same files.

### 4. Pass certification

```go
func TestCertify(t *testing.T) {
    certify.Run(t, certify.Target{
        New: func(t testing.TB, baseURL string) adapterkit.PaymentProvider {
            return New(WithBaseURL(baseURL), WithCredentials("k", "s", "m"))
        },
        ScenariosFile: "vectors/certify.json",
    })
}
```

The required scenario set is derived from your declared capabilities — see
[`SCENARIOS.md`](https://github.com/MonetaKit/monetakit/blob/main/adapterkit/certify/SCENARIOS.md)
in core for IDs and invariants. Your adapter needs a base-URL option so the
harness (and your own tests) can drive it with `httptest` — no network in CI.

### 5. Verify against the sandbox

Add an opt-in, env-gated live test (`<PSP>_LIVE_TEST=1` + credential env
vars; skipped in CI) and date-stamp the successful run in the adapter README.
If you port the PSP's signing scheme, pin golden values generated with the
PSP's official SDK so the port is provably byte-identical.

### 6. Optional: the TS twin (`ts/`)

For PSPs whose flows reach client/edge code, add an npm workspace published
as `@monetakit/<psp>`:

- Zero-dependency and edge-native (Web Crypto + `fetch`), no build step —
  same toolchain as core's `@monetakit/sdk` (`node --experimental-strip-types`
  for tests, MSW for HTTP mocking as a devDependency).
- **Replays the same `vectors/` files** as the Go adapter — charge
  classification, request shapes, and webhook normalization cannot drift.
- Error messages carry the same `<psp>:` prefix and context as the Go side.

### 7. Checklist before opening the PR

- [ ] `make check` and (if `ts/` exists) `npm test` + `npm run typecheck` green
- [ ] certification battery green
- [ ] capability drift test green; every declared fact sourced
- [ ] live sandbox run date-stamped in the adapter README
- [ ] adapter README states scope and any not-production-ready caveats
- [ ] `MAINTAINERS` file
- [ ] `CHANGELOG.md` `[Unreleased]` entry

Review may take a few rounds — automated review runs on every push; we
respond to every comment with an explicit adopt/decline and a reason, and
expect the same of contributors.

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
git clone git@github.com:MonetaKit/contrib.git
cd contrib
make check                                     # Go: fmt + vet + build + test
npm install && npm test && npm run typecheck   # TS workspaces
```

Core is fetched as a normal module (pinned in `go.mod`). To develop against
unreleased core changes, use an uncommitted local override:
`go mod edit -replace github.com/monetakit/monetakit=../monetakit`
