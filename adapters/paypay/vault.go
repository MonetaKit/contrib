package paypay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Vault turns an account-link responseToken into the charge token.
//
// PayPay's "vault" is the account-link flow: the app sends the user through a
// linking QR (POST /v1/qr/sessions with scope "continuous_payments"); PayPay
// then redirects to the merchant with a responseToken JWT. Pass that JWT here
// (as a string): the signature is verified (HS256, keyed with the
// Base64-DECODED API secret, per PayPay's account-link docs), the claims are
// checked (result, audience, expiry), and the embedded userAuthorizationId —
// the token Charge accepts — is returned.
func (a *Adapter) Vault(paymentMethod any) (string, error) {
	jwt, ok := paymentMethod.(string)
	if !ok {
		return "", fmt.Errorf("paypay: Vault expects the account-link responseToken JWT as a string, got %T", paymentMethod)
	}
	claims, err := a.verifyResponseToken(jwt)
	if err != nil {
		return "", err
	}
	if claims.Result != "succeeded" {
		return "", fmt.Errorf("paypay: account link not granted (result %q)", claims.Result)
	}
	if claims.UserAuthorizationID == "" {
		return "", fmt.Errorf("paypay: responseToken carries no userAuthorizationId")
	}
	return claims.UserAuthorizationID, nil
}

type linkClaims struct {
	Result              string `json:"result"`
	UserAuthorizationID string `json:"userAuthorizationId"`
	Audience            string `json:"aud"`
	Issuer              string `json:"iss"`
	ExpiresAt           int64  `json:"exp"`
}

// verifyResponseToken checks an HS256 JWT without a JWT dependency. The alg
// header is pinned to HS256 (rejecting "none"/RS256 confusion), the MAC is
// compared in constant time, and iss/aud/exp are enforced.
func (a *Adapter) verifyResponseToken(token string) (linkClaims, error) {
	var claims linkClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, fmt.Errorf("paypay: responseToken is not a JWT")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, fmt.Errorf("paypay: responseToken header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil || header.Alg != "HS256" {
		return claims, fmt.Errorf("paypay: responseToken alg must be HS256")
	}
	key, err := decodeSecret(a.apiSecret)
	if err != nil {
		return claims, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	want, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(mac.Sum(nil), want) {
		return claims, fmt.Errorf("paypay: responseToken signature mismatch")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, fmt.Errorf("paypay: responseToken payload: %w", err)
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return claims, fmt.Errorf("paypay: responseToken claims: %w", err)
	}
	if claims.Issuer != "paypay.ne.jp" {
		return claims, fmt.Errorf("paypay: responseToken issuer %q is not paypay.ne.jp", claims.Issuer)
	}
	if claims.Audience != a.apiKey {
		return claims, fmt.Errorf("paypay: responseToken audience mismatch")
	}
	if claims.ExpiresAt == 0 {
		return claims, fmt.Errorf("paypay: responseToken has no exp claim — non-expiring link tokens are not accepted")
	}
	if a.now().Unix() >= claims.ExpiresAt {
		return claims, fmt.Errorf("paypay: responseToken expired")
	}
	return claims, nil
}

// decodeSecret Base64-decodes the API secret (the account-link JWT is signed
// with the decoded bytes, unlike request signing which uses the raw string).
// Accepts standard and unpadded encodings.
func decodeSecret(secret string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(secret); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(secret); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("paypay: API secret is not valid base64 (required to verify responseToken)")
}
