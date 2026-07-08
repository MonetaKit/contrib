package paypay

import (
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // MD5 is mandated by PayPay's OPA-Auth protocol (payload digest, not a security boundary)
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// authHeader builds the OPA-Auth HMAC header exactly as PayPay's official SDKs
// do (cross-checked against paypayopa-sdk-node's createAuthHeader; the Go tests
// pin golden values generated with the node implementation):
//
//	digest = Base64(MD5(contentType || body))         // both "empty" when no body
//	raw    = path\nmethod\nnonce\nepoch\ncontentType\ndigest
//	sig    = Base64(HMAC-SHA256(raw, apiSecret))
//	header = hmac OPA-Auth:{apiKey}:{sig}:{nonce}:{epoch}:{digest}
//
// path is the URL path only (no query string, no host); body is the exact
// bytes sent on the wire — the digest must be computed over what is sent.
func authHeader(apiKey, apiSecret, method, path string, body []byte, nonce string, epoch int64) string {
	contentType, digest := "empty", "empty"
	if len(body) > 0 {
		contentType = "application/json"
		h := md5.New() //nolint:gosec // see import note
		h.Write([]byte(contentType))
		h.Write(body)
		digest = base64.StdEncoding.EncodeToString(h.Sum(nil))
	}
	raw := fmt.Sprintf("%s\n%s\n%s\n%d\n%s\n%s", path, method, nonce, epoch, contentType, digest)
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(raw))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("hmac OPA-Auth:%s:%s:%s:%d:%s", apiKey, sig, nonce, epoch, digest)
}
