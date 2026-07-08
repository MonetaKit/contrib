# Contrib PSP adapters

One directory per PSP, lowercase (`tappay/`, `newebpay/`, `adyen/`). Start by
copying `example/` — it compiles against `adapterkit` and marks every decision
you need to make. Full checklist: [CONTRIBUTING.md](../CONTRIBUTING.md).

Each adapter directory is PSP-major — everything about one PSP lives
together, whatever the language:

- `capability.json` — honest capability declaration (checked against
  `Capabilities()` by test)
- `*.go` — the `adapterkit.PaymentProvider` implementation (package at the
  adapter root, so the import path stays `.../adapters/<psp>`)
- `vectors/` — language-neutral conformance vectors (gateway charge
  classification, webhook normalization). The Go tests replay them; a TS
  twin, when one exists, must replay the same files.
- `ts/` — optional TypeScript twin, an npm workspace published as
  `@monetakit/<psp>` (added when a PSP needs client/edge-side code)
- `MAINTAINERS` — GitHub handles responsible for this adapter (all languages)
- `testdata/` — fixtures / VCR cassettes (secrets scrubbed)
