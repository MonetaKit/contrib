package paypay

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/monetakit/monetakit/adapterkit"
)

// createQRRequest field order is part of the signed payload bytes.
type createQRRequest struct {
	MerchantPaymentID string    `json:"merchantPaymentId"`
	Amount            moneyJSON `json:"amount"`
	CodeType          string    `json:"codeType"` // always "ORDER_QR"
	RequestedAt       int64     `json:"requestedAt"`
	OrderDescription  string    `json:"orderDescription,omitempty"`
}

type qrData struct {
	CodeID   string `json:"codeId"`
	URL      string `json:"url"`
	Deeplink string `json:"deeplink"`
}

// chargeQR creates a dynamic QR code the payer scans and pushes money to.
// The result is always "pending" with the payment URL in Action; settlement
// arrives by webhook (state COMPLETED) or polling payment details.
//
// Idempotency caveat: POST /v2/codes does NOT replay duplicates — it returns
// DUPLICATE_DYNAMIC_QR_REQUEST. A retried Charge for the same key therefore
// falls back to GET /v2/codes/payments/{merchantPaymentId} and reports the
// live status (the original QR url cannot be recovered, so Action is empty
// on that path).
func (a *Adapter) chargeQR(amount adapterkit.Money, opts adapterkit.ChargeOpts) (adapterkit.ChargeResult, error) {
	const path = "/v2/codes"
	mpid := merchantPaymentID(opts.IdempotencyKey)
	req := createQRRequest{
		MerchantPaymentID: mpid,
		Amount:            moneyJSON{Amount: amount.Amount, Currency: "JPY"},
		CodeType:          "ORDER_QR",
		RequestedAt:       a.now().Unix(),
		OrderDescription:  opts.Description,
	}
	status, env, err := a.do(http.MethodPost, path, req)
	if err != nil {
		return adapterkit.ChargeResult{}, err
	}
	if status >= 200 && status < 300 {
		var data qrData
		if err := json.Unmarshal(env.Data, &data); err != nil || data.URL == "" {
			return adapterkit.ChargeResult{}, fmt.Errorf("paypay: create code succeeded but no url in response")
		}
		return adapterkit.ChargeResult{
			Status:      "pending",
			ProviderRef: data.CodeID,
			Action:      &adapterkit.ChargeAction{Type: "paypay_qr", Value: data.URL},
		}, nil
	}
	if env.ResultInfo.Code == "DUPLICATE_DYNAMIC_QR_REQUEST" {
		return a.qrPaymentStatus(mpid)
	}
	return adapterkit.ChargeResult{}, apiError(http.MethodPost, path, status, env)
}

// qrPaymentStatus reports the live state of a previously created QR payment.
func (a *Adapter) qrPaymentStatus(merchantPaymentID string) (adapterkit.ChargeResult, error) {
	path := "/v2/codes/payments/" + merchantPaymentID
	status, env, err := a.do(http.MethodGet, path, nil)
	if err != nil {
		return adapterkit.ChargeResult{}, err
	}
	if status < 200 || status >= 300 {
		return adapterkit.ChargeResult{}, apiError(http.MethodGet, path, status, env)
	}
	var data paymentData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return adapterkit.ChargeResult{}, fmt.Errorf("paypay: payment details: %w", err)
	}
	mapped, ok := chargeStatus(data.Status)
	if !ok {
		return adapterkit.ChargeResult{}, fmt.Errorf("paypay: unknown payment status %q", data.Status)
	}
	return adapterkit.ChargeResult{Status: mapped, ProviderRef: data.PaymentID}, nil
}
