# Contrib PSP adapters

One directory per PSP, lowercase (`tappay/`, `newebpay/`, `adyen/`). Start by
copying `example/` — it compiles against `adapterkit` and marks every decision
you need to make. Full checklist: [CONTRIBUTING.md](../CONTRIBUTING.md).

Each adapter directory contains:

- `capability.json` — honest capability declaration (checked against
  `Capabilities()` by test)
- `<psp>.go` — the `adapterkit.PaymentProvider` implementation
- `<psp>_*_test.go` — conformance vectors + VCR-recorded API tests
- `MAINTAINERS` — GitHub handles responsible for this adapter
- `testdata/` — VCR cassettes (secrets scrubbed)
