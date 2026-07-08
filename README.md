# MonetaKit Contrib

Community-maintained extensions for [MonetaKit](https://github.com/MonetaKit/monetakit),
hosted by the MonetaKit org: PSP adapters beyond the core set, and country-specific
invoicing and tax-filing formats.

## Support tiers

| Tier | Lives in | Maintained by | Gate |
|---|---|---|---|
| **core** | [`monetakit/monetakit`](https://github.com/MonetaKit/monetakit) (`adapters/`) | MonetaKit team | — |
| **contrib** | this repo | community, org-reviewed | conformance suite green + code review |
| **community** | author's own repo | author | listed in the docs registry, conformance badge self-reported |

Everything in this repo is community-supported: the MonetaKit team reviews and
merges, but does not guarantee fixes on any timeline. An adapter that becomes
load-bearing and well-maintained can graduate to core; an unmaintained one gets
archived rather than silently shipped.

## Layout

```
adapters/    PSP adapters (Go — implement adapterkit.PaymentProvider)
│            └── example/   compiling skeleton to copy from
invoicing/   country e-invoice formats (data-first: schemas, mappings, vectors)
tax/         tax-filing / reporting formats (data-first)
```

## How adapters ship

Adapters run in-process with the engine/CLI, so they are Go and implement the
[`adapterkit.PaymentProvider`](https://github.com/MonetaKit/monetakit/blob/main/adapterkit/provider.go)
contract. Current integration path: once an adapter here passes conformance, a
small PR in core wires it into the `moneta` provider switch and it ships in the
official binary, marked community-supported in the docs. (A proper factory
registry in `adapterkit` — and possibly out-of-process plugins — may replace
this later; the contract you implement stays the same.)

## Developing

Core has no tagged release yet, so this repo uses a `replace` directive that
expects a sibling checkout:

```bash
git clone git@github.com:MonetaKit/monetakit.git
git clone git@github.com:MonetaKit/contrib.git
cd contrib && make check
```

Once core tags `v0.x`, the `replace` goes away and `go.mod` pins a version.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Short version:

- **PSP adapter** — copy `adapters/example/`, implement the interface, declare
  honest `capability.json`, pass the shared conformance vectors.
- **Invoicing / tax format** — you don't need to write Go. Contribute the spec
  as data: field mappings, schemas, and golden test vectors. See
  `invoicing/README.md`.

## License

[MIT](LICENSE), same as core.
