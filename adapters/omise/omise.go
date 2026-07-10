// Package omise is a contrib-tier MonetaKit adapter for Omise / Opn Payments,
// a Thailand-focused PSP. This adapter's scope is Omise's PromptPay push rail:
// PromptPay is Thailand's national QR payment scheme (bank-to-bank, proxy-
// addressed) and Omise is the acquirer that turns it into a programmable API —
// it registers as the biller, mints a dynamic QR per charge, and notifies us
// by webhook. We are NOT a PromptPay biller ourselves; the QR always comes from
// Omise.
//
//   - Push (QR): Charge with currency THB creates a PromptPay source charge
//     (POST /charges, source[type]=promptpay) and returns Status "pending"
//     with the scannable QR in ChargeResult.Action. Settlement arrives by
//     webhook (charge.complete) — see webhook.go.
//
// Out of scope for this adapter (separate follow-ups):
//   - Omise's card pull rail (tokens/customers) — would add chargeMode "pull"
//     and Vault. Declared push-only here so certification does not demand the
//     pull battery.
//   - Recurring: self-managed only (recurringEngine "none").
//
// No catalog: Omise has no MonetaKit-shaped product/price objects, so
// Read/Diff/Apply are no-ops and every providerId is unmanaged.
//
// Currency: THB only. Amounts are int64 satang (1 THB = 100 satang), which is
// adapterkit's smallest-unit convention already.
//
// ⚠️ Omise webhooks are UNSIGNED (see ParseWebhook) — treat every event as a
// hint and confirm by API lookup (GET /charges/{id}) before fulfilling.
package omise

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/monetakit/monetakit/adapterkit"
	"github.com/monetakit/monetakit/planfile"
)

const (
	// Omise's charge/source API. (vault.omise.co is card-only; PromptPay never
	// touches it.)
	productionBaseURL = "https://api.omise.co"
	// Omise has no separate sandbox host — the test/live split is the API key
	// prefix (skey_test_ vs skey_). Kept as a named constant for symmetry with
	// other adapters and in case that changes.
	sandboxBaseURL = "https://api.omise.co"
)

type Adapter struct {
	http      *http.Client
	baseURL   string
	secretKey string // skey_test_… / skey_… — HTTP Basic username, empty password
}

type Option func(*Adapter)

func WithHTTPClient(c *http.Client) Option { return func(a *Adapter) { a.http = c } }
func WithBaseURL(u string) Option          { return func(a *Adapter) { a.baseURL = u } }
func WithSandbox() Option                  { return func(a *Adapter) { a.baseURL = sandboxBaseURL } }

// WithCredentials sets the Omise secret key. Only the secret key is needed
// server-side; the public key is a browser/edge concern (the optional ts twin).
func WithCredentials(secretKey string) Option {
	return func(a *Adapter) { a.secretKey = secretKey }
}

// New reads OMISE_SECRET_KEY from the environment (options override).
func New(opts ...Option) *Adapter {
	a := &Adapter{
		http:      http.DefaultClient,
		baseURL:   productionBaseURL,
		secretKey: os.Getenv("OMISE_SECRET_KEY"),
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (a *Adapter) Name() string { return "omise" }

// Capabilities must match capability.json (the drift test checks).
func (a *Adapter) Capabilities() adapterkit.Capabilities {
	return adapterkit.Capabilities{
		RecurringEngine: "none", // self-managed engine bills; Omise has no native subscriptions here
		HasCatalog:      false,  // no product/price objects; providerIds always unmanaged
		TieredPricing:   "none",
		ChargeModes:     []string{"push"}, // PromptPay QR only in this adapter
		// THB only. Minimum 2000 satang (฿20), sourced from Omise's own
		// invalid_charge error ("amount must be greater than or equal to ฿20
		// (2000 satangs)"), verified against the test API 2026-07-09. No
		// maximum is documented, so none is declared.
		Currencies: map[string]adapterkit.CurrencyLimit{"thb": {Min: 2000}},
	}
}

// --- HTTP plumbing ---

// omiseError is Omise's error object ({"object":"error",...}) — a transport /
// request-level failure, distinct from a charge that was created but failed.
type omiseError struct {
	Object  string `json:"object"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// charge is the subset of Omise's charge object this adapter reads.
type charge struct {
	Object   string `json:"object"`
	ID       string `json:"id"`
	Status   string `json:"status"` // pending | successful | failed | expired | reversed
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Failure  string `json:"failure_code"`
	Source   struct {
		ScannableCode struct {
			Type  string `json:"type"`
			Image struct {
				DownloadURI string `json:"download_uri"`
			} `json:"image"`
		} `json:"scannable_code"`
	} `json:"source"`
}

// post sends a form-encoded Omise request (Omise takes form params, returns
// JSON). idempotencyKey, when non-empty, becomes the Idempotency-Key header so
// a retried period-charge never double-collects.
func (a *Adapter) post(path string, form url.Values, idempotencyKey string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, a.baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, fmt.Errorf("omise: POST %s: %w", path, err)
	}
	req.SetBasicAuth(a.secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("omise: POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("omise: POST %s: read body: %w", path, err)
	}
	return resp.StatusCode, raw, nil
}

// --- Charge (the gateway primitive) ---

// Charge creates a PromptPay QR charge for amount (THB only) and returns
// Status "pending" with the scannable QR in Action{Type:"promptpay_qr"};
// settlement arrives via webhook (charge.complete).
//
// token is ignored — PromptPay is push, there is no vaulted instrument to pull.
//
// opts.IdempotencyKey is required and is sent verbatim as Omise's
// Idempotency-Key header. ⚠️ The Omise test account observed (API version
// 2019-05-29, 2026-07-09) did NOT dedup on this header — a repeated key minted
// a second charge. Because PromptPay is push, that is NOT a double-collect:
// Charge only creates a QR (no money moves), so a lease-retry yields a second
// UNPAID QR that expires; the payer scans and pays at most one. The consumer
// reconciles by the charge id in the paid webhook, not by assuming Omise
// deduplicates. The key is still sent (verbatim, never sanitized) so dedup
// takes effect automatically if the account is on a version that honors it.
func (a *Adapter) Charge(token string, amount adapterkit.Money, opts adapterkit.ChargeOpts) (adapterkit.ChargeResult, error) {
	if !strings.EqualFold(amount.Currency, "thb") {
		return adapterkit.ChargeResult{}, fmt.Errorf("omise: only thb is supported, got %q", amount.Currency)
	}
	if opts.IdempotencyKey == "" {
		return adapterkit.ChargeResult{}, fmt.Errorf("omise: ChargeOpts.IdempotencyKey is required (sent as the Idempotency-Key header)")
	}

	form := url.Values{}
	form.Set("amount", strconv.FormatInt(amount.Amount, 10))
	form.Set("currency", "thb")
	form.Set("source[type]", "promptpay")
	if opts.Description != "" {
		form.Set("description", opts.Description)
	}

	status, raw, err := a.post("/charges", form, opts.IdempotencyKey)
	if err != nil {
		return adapterkit.ChargeResult{}, err
	}
	if status < 200 || status >= 300 {
		var e omiseError
		_ = json.Unmarshal(raw, &e)
		return adapterkit.ChargeResult{}, fmt.Errorf("omise: POST /charges -> %d %s: %s", status, e.Code, e.Message)
	}

	var c charge
	if err := json.Unmarshal(raw, &c); err != nil {
		return adapterkit.ChargeResult{}, fmt.Errorf("omise: POST /charges -> %d: decode charge: %w", status, err)
	}

	switch c.Status {
	case "pending":
		// The QR the payer scans. Omise hands us an image download URI, not the
		// raw EMV payload, so that URI is the actionable value.
		qr := c.Source.ScannableCode.Image.DownloadURI
		if qr == "" {
			return adapterkit.ChargeResult{}, fmt.Errorf("omise: charge %s is pending but carries no scannable_code — cannot present a QR", c.ID)
		}
		return adapterkit.ChargeResult{
			Status:      "pending",
			ProviderRef: c.ID,
			Action:      &adapterkit.ChargeAction{Type: "promptpay_qr", Value: qr},
		}, nil
	case "successful":
		return adapterkit.ChargeResult{Status: "succeeded", ProviderRef: c.ID}, nil
	case "failed", "expired", "reversed":
		// A business decline (the payer cannot pay) — a result that consumes a
		// dunning attempt, not a transport error the scheduler lease-retries.
		return adapterkit.ChargeResult{Status: "failed", ProviderRef: c.ID}, nil
	default:
		return adapterkit.ChargeResult{}, fmt.Errorf("omise: unknown charge status %q (charge %s)", c.Status, c.ID)
	}
}

// Vault is not supported: PromptPay is a push rail with no vaulted instrument
// to store. (Omise's card pull rail — a future adapter/mode — is where Vault
// would live.)
func (a *Adapter) Vault(paymentMethod any) (string, error) {
	return "", fmt.Errorf("omise: PromptPay is push-only; no payment method to vault")
}

// --- Catalog no-ops (hasCatalog=false) ---

func (a *Adapter) Read() (adapterkit.CanonicalState, error) {
	return adapterkit.CanonicalState{}, nil
}

func (a *Adapter) Diff(desired planfile.IR, live adapterkit.CanonicalState) adapterkit.Plan {
	return adapterkit.Plan{Provider: a.Name()}
}

func (a *Adapter) Apply(desired planfile.IR, live adapterkit.CanonicalState, plan adapterkit.Plan) (adapterkit.ApplyResult, error) {
	return adapterkit.ApplyResult{}, nil
}

var _ adapterkit.PaymentProvider = (*Adapter)(nil)
