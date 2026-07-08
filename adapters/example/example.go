// Package example is a compiling skeleton for a contrib PSP adapter.
// Copy this directory to adapters/<psp>/, rename the package, and replace
// every errNotImplemented with a real call to the PSP's API.
//
// Read adapterkit/provider.go in core first — it is the contract, and its
// comments state the invariants (idempotency keys, pac.* metadata, archive-
// never-delete) that conformance enforces.
package example

import (
	"errors"

	"github.com/monetakit/monetakit/adapterkit"
	"github.com/monetakit/monetakit/planfile"
)

// Compile-time proof the contract is fully implemented. Optional capabilities
// (adapterkit.UsageReporter, WebhookParser, DriftDetector, SubscriptionCounter)
// get their own assertion lines when you implement them.
var _ adapterkit.PaymentProvider = (*Provider)(nil)

var errNotImplemented = errors.New("example: not implemented")

// Provider holds the PSP client and credentials. Construct it in New so the
// CLI can wire it with one call.
type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "example" }

// Capabilities must match capability.json in this directory (the test checks).
// Declare honestly: moneta plan uses this to reject unsupported Planfiles at
// plan-time instead of failing mid-apply.
func (p *Provider) Capabilities() adapterkit.Capabilities {
	return adapterkit.Capabilities{
		RecurringEngine: "none", // no native subscriptions => self-managed engine
		HasCatalog:      false,  // no catalog => Read/Diff/Apply are mostly no-ops
		TieredPricing:   "none",
	}
}

// Charge moves money. It MUST pass opts.IdempotencyKey to the PSP so a
// retried period never double-charges — conformance/engine pins the key shape.
func (p *Provider) Charge(token string, amount adapterkit.Money, opts adapterkit.ChargeOpts) (adapterkit.ChargeResult, error) {
	return adapterkit.ChargeResult{}, errNotImplemented
}

// Vault stores a payment method for off-session reuse and returns the token
// Charge accepts.
func (p *Provider) Vault(paymentMethod any) (string, error) {
	return "", errNotImplemented
}

// Read returns the live catalog in IR terms. Catalog-less PSPs return an
// empty state (every providerId is then unmanaged).
func (p *Provider) Read() (adapterkit.CanonicalState, error) {
	return adapterkit.CanonicalState{}, nil
}

// Diff compares desired IR against live state and emits the change plan.
func (p *Provider) Diff(desired planfile.IR, live adapterkit.CanonicalState) adapterkit.Plan {
	return adapterkit.Plan{Provider: p.Name()}
}

// Apply executes the plan. Invariants: immutable price version chains,
// archive-never-delete, and safe re-runs (partial failure must be resumable).
func (p *Provider) Apply(desired planfile.IR, live adapterkit.CanonicalState, plan adapterkit.Plan) (adapterkit.ApplyResult, error) {
	return adapterkit.ApplyResult{}, errNotImplemented
}
