# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Go adapters version with the repo tag; each `adapters/*/ts` npm package
versions independently.

## [Unreleased]

### Added

- `adapters/omise`: Omise / Opn Payments adapter scoped to the **PromptPay**
  push rail (Thailand) — THB charge → `pending` + `promptpay_qr`
  `ChargeResult.Action`, `charge.complete` webhook normalization (unsigned;
  fails closed, confirm by API lookup). Declares `chargeModes: ["push"]` and
  `currencies: {thb}`; gateway + webhook vectors and the `adapterkit/certify`
  battery replay offline. Live-sandbox run and PromptPay charge-bound
  declaration still TODO (see the adapter README).
- `adapters/paypay`: PayPay (Japan) adapter — QR push (`ChargeResult.Action`)
  + continuous-payment pull, unsigned-webhook normalization (fails closed),
  responseToken `Vault`; OPA-Auth signing pinned to official-SDK golden
  vectors and verified against the live sandbox
  ([#1](https://github.com/MonetaKit/contrib/pull/1)).
- `adapters/paypay/ts`: `@monetakit/paypay` TS twin (zero-dependency,
  edge-native, MSW-tested) replaying the same `vectors/` files as the Go
  adapter; npm workspaces (`adapters/*/ts`) + TS CI job established
  ([#1](https://github.com/MonetaKit/contrib/pull/1)).
- `adapters/paypay`: certified against core's `adapterkit/certify` battery
  (v0.0.2) — declares `chargeModes: ["pull", "push"]` and `currencies: {jpy}`;
  first adapter through the full push+pull scenario set
  ([#1](https://github.com/MonetaKit/contrib/pull/1)).
- Repo scaffolding: contribution tiers (core / contrib / community), the
  `adapters/example` skeleton with a capability-drift test, and data-first
  conventions for `invoicing/` and `tax/` formats.
- CI: Go `make check` against core checked out as a module dependency.

### Changed

- README and CONTRIBUTING rewritten around the shipped reality: the
  declare → enforce → certify quality model, PSP-major layout with TS twins
  and shared vectors, the pre-PR checklist, and versioning/release policy.
- Pinned core `v0.0.1` as the adapterkit compatibility anchor and dropped the
  sibling-checkout `replace` once core went public.
- CI runs on current action majors (checkout v7, setup-go v6).
