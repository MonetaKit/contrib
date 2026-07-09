# MonetaKit Contrib

Community-maintained extensions for [MonetaKit](https://github.com/MonetaKit/monetakit),
hosted by the MonetaKit org: PSP adapters beyond the core set, and country-specific
invoicing and tax-filing formats.

## Support tiers

| Tier | Lives in | Maintained by | Gate |
|---|---|---|---|
| **core** | [`monetakit/monetakit`](https://github.com/MonetaKit/monetakit) (`adapters/`) | MonetaKit team | — |
| **contrib** | this repo | community, org-reviewed | certification battery green + code review |
| **community** | author's own repo | author | listed in the docs registry, certification self-reported |

Everything in this repo is community-supported: the MonetaKit team reviews and
merges, but does not guarantee fixes on any timeline. An adapter that becomes
load-bearing and well-maintained can graduate to core; an unmaintained one gets
archived rather than silently shipped.

## Layout

One directory per PSP, everything about that PSP together — whatever the
language ([why](adapters/README.md)):

```
adapters/
├── example/         minimal compiling skeleton (copy to start)
└── paypay/          the full reference implementation
    ├── *.go         Go adapter (implements adapterkit.PaymentProvider)
    ├── capability.json
    ├── vectors/     language-neutral fixtures: gateway, webhooks, certify
    ├── ts/          TS twin — npm workspace @monetakit/paypay
    └── MAINTAINERS
invoicing/           country e-invoice formats (data-first)
tax/                 tax-filing / reporting formats (data-first)
```

## Quality model: declare → enforce → certify

An adapter declares what its PSP supports in `capability.json` /
`Capabilities()` (currencies and charge bounds, charge modes, catalog,
metering…). Core enforces those declarations once for everyone (`moneta plan`
rejects incompatible Planfiles at plan-time). The **certification battery**
(`adapterkit/certify` in core) then derives a test suite from the declarations:

- **Laws** — invariants checked automatically: capability vocabulary, declared
  ⇔ implemented coherence, catalog-less no-ops, webhooks fail closed,
  undeclared currencies rejected before the network.
- **Scenarios** — named wire fixtures you supply (`CHG-01` success, `CHG-03`
  decline, `CHG-04` transport failure, `PSH-01` push action, …); the harness
  owns the assertions.

The catalog lives in core:
[`adapterkit/certify/SCENARIOS.md`](https://github.com/MonetaKit/monetakit/blob/main/adapterkit/certify/SCENARIOS.md).
**"Certify green" is the merge gate for adapters.**

## How adapters ship

Adapters run in-process with the engine/CLI, so the charge path is Go and
implements the
[`adapterkit.PaymentProvider`](https://github.com/MonetaKit/monetakit/blob/main/adapterkit/provider.go)
contract. Once certified and merged here, a small PR in core wires the adapter
into the `moneta` provider switch and it ships in the official binary, marked
community-supported. A TS twin (`adapters/<psp>/ts`, published as
`@monetakit/<psp>`) is optional and adds client/edge-side support — it must
replay the same `vectors/` files as the Go adapter so the two cannot drift.

## Developing

`go.mod` pins core's tagged release (currently `v0.0.2`, the adapterkit
compatibility anchor) — a plain `go build` fetches it like any module:

```bash
git clone git@github.com:MonetaKit/contrib.git
cd contrib
make check    # Go: fmt + vet + build + test
npm install && npm test && npm run typecheck   # TS workspaces (adapters/*/ts)
```

To develop against unreleased core changes, add a local override (don't
commit it): `go mod edit -replace github.com/monetakit/monetakit=../monetakit`

## Versioning & releases

- **Go adapters** version together with this repo's tag.
- **TS packages** (`adapters/*/ts`) are independent npm workspaces under the
  `@monetakit` scope. Nothing is published to npm yet — the publish pipeline
  is tracked in [monetakit#6](https://github.com/MonetaKit/monetakit/issues/6).
- Every PR updates `[Unreleased]` in [CHANGELOG.md](CHANGELOG.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Short version:

- **PSP adapter** — start from `adapters/example/`, model the result on
  `adapters/paypay/`; declare honest capabilities; pass the certification
  battery; ship language-neutral vectors.
- **Invoicing / tax format** — you don't need to write Go. Contribute the
  spec as data: field mappings, schemas, and golden test vectors. See
  `invoicing/README.md`.

## License

[MIT](LICENSE), same as core.
