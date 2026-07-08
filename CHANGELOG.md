# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Go adapters version with the repo tag; each `adapters/*/ts` npm package
versions independently.

## [Unreleased]

### Added

- `adapters/paypay`: PayPay (Japan) adapter — QR push (`ChargeResult.Action`)
  + continuous-payment pull, unsigned-webhook normalization (fails closed),
  responseToken `Vault`; OPA-Auth signing pinned to official-SDK golden
  vectors and verified against the live sandbox
  ([#1](https://github.com/MonetaKit/contrib/pull/1)).
- `adapters/paypay/ts`: `@monetakit/paypay` TS twin (zero-dependency,
  edge-native, MSW-tested) replaying the same `vectors/` files as the Go
  adapter; npm workspaces (`adapters/*/ts`) + TS CI job established
  ([#1](https://github.com/MonetaKit/contrib/pull/1)).
- Repo scaffolding: contribution tiers (core / contrib / community), the
  `adapters/example` skeleton with a capability-drift test, and data-first
  conventions for `invoicing/` and `tax/` formats.
- CI: Go `make check` against core checked out as a module dependency.

### Changed

- Pinned core `v0.0.1` as the adapterkit compatibility anchor and dropped the
  sibling-checkout `replace` once core went public.
- CI runs on current action majors (checkout v7, setup-go v6).
