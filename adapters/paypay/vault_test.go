package paypay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// signJWT builds an HS256 JWT the way PayPay signs responseTokens (key = the
// Base64-DECODED API secret).
func signJWT(t *testing.T, key []byte, header, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(header)
	c, _ := json.Marshal(claims)
	signing := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signing))
	return signing + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestVault(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	apiSecret := base64.StdEncoding.EncodeToString(key)
	newAdapter := func() *Adapter {
		a := New(WithCredentials("client_1", apiSecret, "m1"))
		a.now = func() time.Time { return time.Unix(1700000000, 0) }
		return a
	}
	hs256 := map[string]any{"alg": "HS256", "typ": "JWT"}
	claims := func(overrides map[string]any) map[string]any {
		c := map[string]any{
			"result": "succeeded", "userAuthorizationId": "ua_9",
			"aud": "client_1", "iss": "paypay.ne.jp", "exp": 1700003600,
		}
		for k, v := range overrides {
			c[k] = v
		}
		return c
	}

	t.Run("valid token yields the userAuthorizationId", func(t *testing.T) {
		got, err := newAdapter().Vault(signJWT(t, key, hs256, claims(nil)))
		if err != nil || got != "ua_9" {
			t.Errorf("got %q, %v", got, err)
		}
	})

	// Tamper: swap the claims segment after signing — the MAC must not verify.
	valid := signJWT(t, key, hs256, claims(nil))
	forged, _ := json.Marshal(claims(map[string]any{"userAuthorizationId": "ua_ATTACKER"}))
	parts := strings.Split(valid, ".")
	tampered := parts[0] + "." + base64.RawURLEncoding.EncodeToString(forged) + "." + parts[2]

	rejects := map[string]string{
		"tampered claims":   tampered,
		"alg none":          signJWT(t, key, map[string]any{"alg": "none"}, claims(nil)),
		"wrong signing key": signJWT(t, []byte("wrong-key"), hs256, claims(nil)),
		"expired":           signJWT(t, key, hs256, claims(map[string]any{"exp": 1600000000})),
		"audience mismatch": signJWT(t, key, hs256, claims(map[string]any{"aud": "someone_else"})),
		"wrong issuer":      signJWT(t, key, hs256, claims(map[string]any{"iss": "evil.example"})),
		"user declined":     signJWT(t, key, hs256, claims(map[string]any{"result": "declined"})),
		"not a JWT":         "not-a-jwt",
	}
	for name, token := range rejects {
		t.Run(name, func(t *testing.T) {
			if got, err := newAdapter().Vault(token); err == nil {
				t.Errorf("want error, got token %q", got)
			}
		})
	}

	t.Run("non-string payment method", func(t *testing.T) {
		if _, err := newAdapter().Vault(42); err == nil {
			t.Error("want error for non-string")
		}
	})
}
