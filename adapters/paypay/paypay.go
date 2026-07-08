// Package paypay is a contrib-tier MonetaKit adapter for PayPay (Japan's
// dominant QR wallet) via the Open Payment API (OPA).
//
// PayPay is a hybrid rail. This adapter implements both sides:
//
//   - Pull (off-session): Charge with token = a userAuthorizationId obtained
//     from PayPay's account-link flow charges the linked wallet via
//     POST /v1/subscription/payments — the gateway primitive the self-managed
//     engine (planfiled) bills subscription periods with.
//   - Push (QR): Charge with token = "qr" creates a dynamic QR code
//     (POST /v2/codes) and returns Status "pending" with the payment URL in
//     ChargeResult.Action; settlement arrives by webhook / polling.
//
// No catalog: PayPay has no product/price objects, so Read/Diff/Apply are
// no-ops and every providerId is unmanaged. Recurring is self-managed only
// (recurringEngine "none").
//
// Currency: JPY only (zero-decimal — adapterkit's smallest-unit int64 is
// already whole yen).
//
// ⚠️ PayPay webhooks are UNSIGNED (see ParseWebhook) — treat events as hints
// and confirm by API lookup before fulfilling.
package paypay

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/monetakit/monetakit/adapterkit"
	"github.com/monetakit/monetakit/planfile"
)

const (
	productionBaseURL = "https://api.paypay.ne.jp"
	sandboxBaseURL    = "https://stg-api.sandbox.paypay.ne.jp"
)

type Adapter struct {
	http       *http.Client
	baseURL    string
	apiKey     string
	apiSecret  string
	merchantID string

	now   func() time.Time // injectable clock (tests)
	nonce func() string    // injectable nonce (tests)
}

type Option func(*Adapter)

func WithHTTPClient(c *http.Client) Option { return func(a *Adapter) { a.http = c } }
func WithBaseURL(u string) Option          { return func(a *Adapter) { a.baseURL = u } }
func WithSandbox() Option                  { return func(a *Adapter) { a.baseURL = sandboxBaseURL } }
func WithCredentials(apiKey, apiSecret, merchantID string) Option {
	return func(a *Adapter) { a.apiKey, a.apiSecret, a.merchantID = apiKey, apiSecret, merchantID }
}

// New reads PAYPAY_API_KEY / PAYPAY_API_SECRET / PAYPAY_MERCHANT_ID from the
// environment (options override). PAYPAY_ENVIRONMENT=sandbox selects the
// sandbox host; production is the default.
func New(opts ...Option) *Adapter {
	a := &Adapter{
		http:       http.DefaultClient,
		baseURL:    productionBaseURL,
		apiKey:     os.Getenv("PAYPAY_API_KEY"),
		apiSecret:  os.Getenv("PAYPAY_API_SECRET"),
		merchantID: os.Getenv("PAYPAY_MERCHANT_ID"),
		now:        time.Now,
		nonce:      randomNonce,
	}
	if os.Getenv("PAYPAY_ENVIRONMENT") == "sandbox" {
		a.baseURL = sandboxBaseURL
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func randomNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return hex.EncodeToString(b[:])
}

func (a *Adapter) Name() string { return "paypay" }

// Capabilities must match capability.json (the test checks).
func (a *Adapter) Capabilities() adapterkit.Capabilities {
	return adapterkit.Capabilities{
		RecurringEngine: "none", // merchant-triggered continuous payments => self-managed engine
		HasCatalog:      false,  // no product/price objects; providerIds always unmanaged
		TieredPricing:   "none",
		ChargeModes:     []string{"pull", "push"}, // continuous payments + dynamic QR
		// JPY only. No charge bounds declared yet: the OPA spec caps amount at
		// 11 digits but documents no universal min/max — don't invent one.
		Currencies: map[string]adapterkit.CurrencyLimit{"jpy": {}},
	}
}

// --- HTTP plumbing ---

// envelope is OPA's uniform response shape.
type envelope struct {
	ResultInfo struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		CodeID  string `json:"codeId"`
	} `json:"resultInfo"`
	Data json.RawMessage `json:"data"`
}

// do sends a signed OPA request. body nil => GET/DELETE-style empty signature.
func (a *Adapter) do(method, path string, reqBody any) (int, envelope, error) {
	var env envelope
	var bodyBytes []byte
	if reqBody != nil {
		var err error
		if bodyBytes, err = json.Marshal(reqBody); err != nil {
			return 0, env, fmt.Errorf("paypay: marshal %s %s: %w", method, path, err)
		}
	}
	req, err := http.NewRequest(method, a.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, env, fmt.Errorf("paypay: %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization",
		authHeader(a.apiKey, a.apiSecret, method, path, bodyBytes, a.nonce(), a.now().Unix()))
	if a.merchantID != "" {
		req.Header.Set("X-ASSUME-MERCHANT", a.merchantID)
	}
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return 0, env, fmt.Errorf("paypay: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, env, fmt.Errorf("paypay: %s %s: read body: %w", method, path, err)
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return resp.StatusCode, env, fmt.Errorf("paypay: %s %s -> %d: non-OPA body: %w", method, path, resp.StatusCode, err)
	}
	return resp.StatusCode, env, nil
}

// apiError formats a non-2xx OPA envelope, including the code needed for
// PayPay's error-resolve page.
func apiError(method, path string, status int, env envelope) error {
	return fmt.Errorf("paypay: %s %s -> %d %s (codeId %s): %s",
		method, path, status, env.ResultInfo.Code, env.ResultInfo.CodeID, env.ResultInfo.Message)
}

// --- Charge (the gateway primitive) ---

type moneyJSON struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// continuousChargeRequest field order is pinned by the auth golden test (the
// payload digest is computed over these exact bytes).
type continuousChargeRequest struct {
	MerchantPaymentID   string    `json:"merchantPaymentId"`
	UserAuthorizationID string    `json:"userAuthorizationId"`
	Amount              moneyJSON `json:"amount"`
	RequestedAt         int64     `json:"requestedAt"`
	OrderDescription    string    `json:"orderDescription,omitempty"`
}

type paymentData struct {
	PaymentID           string `json:"paymentId"`
	Status              string `json:"status"`
	MerchantPaymentID   string `json:"merchantPaymentId"`
	UserAuthorizationID string `json:"userAuthorizationId"`
}

// declineCodes are business outcomes (the wallet/user cannot pay) — a
// ChargeResult{Status:"failed"} that consumes a dunning attempt, not a
// transport error for the scheduler's lease-retry path. From the OPA
// continuous-payments error table.
var declineCodes = map[string]bool{
	"NO_SUFFICIENT_FUND":                     true,
	"LIMIT_EXCEEDED":                         true,
	"USER_DEFINED_DAILY_LIMIT_EXCEEDED":      true,
	"USER_DEFINED_MONTHLY_LIMIT_EXCEEDED":    true,
	"USER_DAILY_LIMIT_FOR_MERCHANT_EXCEEDED": true,
	"NON_KYC_USER":                           true,
	"CANCELED_USER":                          true,
	"NO_VALID_PAYMENT_METHOD":                true,
	"CC_LIMIT_EXCEEDED":                      true,
	"PPC_EXPIRED":                            true,
	"PPC_LIMIT_EXCEEDED":                     true,
	"PAY_METHOD_INVALIDATED":                 true,
	"USER_STATE_IS_NOT_ACTIVE":               true,
	"INVALID_USER_AUTHORIZATION_ID":          true,
	"EXPIRED_USER_AUTHORIZATION_ID":          true,
	"SUSPECTED_DUPLICATE_PAYMENT":            true,
}

// chargeStatus maps OPA payment status enums to the neutral ChargeResult status.
func chargeStatus(s string) (string, bool) {
	switch s {
	case "COMPLETED", "REFUNDED": // REFUNDED = an idempotent replay of a charge refunded later
		return "succeeded", true
	case "CREATED", "AUTHORIZED", "REAUTHORIZING":
		return "pending", true
	case "FAILED", "CANCELED", "EXPIRED":
		return "failed", true
	}
	return "", false
}

// Charge collects amount (JPY only) from a PayPay user.
//
// token forms:
//   - a userAuthorizationId (from Vault / account link): off-session
//     continuous payment — the self-managed engine's billing path.
//   - "qr": dynamic QR push payment — returns Status "pending" with the
//     payment URL in Action{Type:"paypay_qr"}; settlement arrives via
//     webhook/polling.
//
// opts.IdempotencyKey is required; it becomes merchantPaymentId (sanitized —
// see merchantPaymentID). PayPay replays the previous result for a duplicate
// continuous-payment merchantPaymentId, so retried period-charges are safe.
func (a *Adapter) Charge(token string, amount adapterkit.Money, opts adapterkit.ChargeOpts) (adapterkit.ChargeResult, error) {
	if !strings.EqualFold(amount.Currency, "jpy") {
		return adapterkit.ChargeResult{}, fmt.Errorf("paypay: only jpy is supported, got %q", amount.Currency)
	}
	if opts.IdempotencyKey == "" {
		return adapterkit.ChargeResult{}, fmt.Errorf("paypay: ChargeOpts.IdempotencyKey is required (it becomes merchantPaymentId)")
	}
	if token == "qr" {
		return a.chargeQR(amount, opts)
	}
	return a.chargeContinuous(token, amount, opts)
}

func (a *Adapter) chargeContinuous(userAuthorizationID string, amount adapterkit.Money, opts adapterkit.ChargeOpts) (adapterkit.ChargeResult, error) {
	const path = "/v1/subscription/payments"
	req := continuousChargeRequest{
		MerchantPaymentID:   merchantPaymentID(opts.IdempotencyKey),
		UserAuthorizationID: userAuthorizationID,
		Amount:              moneyJSON{Amount: amount.Amount, Currency: "JPY"},
		RequestedAt:         a.now().Unix(),
		OrderDescription:    opts.Description,
	}
	status, env, err := a.do(http.MethodPost, path, req)
	if err != nil {
		return adapterkit.ChargeResult{}, err
	}
	var data paymentData
	_ = json.Unmarshal(env.Data, &data) // data may be null on errors
	if status >= 200 && status < 300 {
		mapped, ok := chargeStatus(data.Status)
		if !ok {
			return adapterkit.ChargeResult{}, fmt.Errorf("paypay: unknown payment status %q (paymentId %s)", data.Status, data.PaymentID)
		}
		return adapterkit.ChargeResult{Status: mapped, ProviderRef: data.PaymentID}, nil
	}
	if declineCodes[env.ResultInfo.Code] {
		return adapterkit.ChargeResult{Status: "failed", ProviderRef: data.PaymentID}, nil
	}
	return adapterkit.ChargeResult{}, apiError(http.MethodPost, path, status, env)
}

// merchantPaymentID makes an idempotency key valid for PayPay: ≤64 chars of
// [a-zA-Z0-9_-]. Engine keys ("sub:periodStart:attempt") contain colons, so
// invalid keys map deterministically to sanitized[:40] + "-" + sha256[:16] —
// the hash of the ORIGINAL key keeps distinct keys distinct after sanitizing.
func merchantPaymentID(key string) string {
	if validMerchantPaymentID.MatchString(key) {
		return key
	}
	sanitized := invalidMPIDChar.ReplaceAllString(key, "-")
	if len(sanitized) > 40 {
		sanitized = sanitized[:40]
	}
	sum := sha256.Sum256([]byte(key))
	return sanitized + "-" + hex.EncodeToString(sum[:])[:16]
}

var (
	validMerchantPaymentID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	invalidMPIDChar        = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
)

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
