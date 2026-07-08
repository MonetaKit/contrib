# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Go adapters version with the repo tag; each `adapters/*/ts` npm package
versions independently.

## [Unreleased]

### Added

- Repo scaffolding: contribution tiers (core / contrib / community), the
  `adapters/example` skeleton with a capability-drift test, and data-first
  conventions for `invoicing/` and `tax/` formats.
- CI: Go `make check` against core checked out as a module dependency.

### Changed

- Pinned core `v0.0.1` as the adapterkit compatibility anchor and dropped the
  sibling-checkout `replace` once core went public.
- CI runs on current action majors (checkout v7, setup-go v6).
