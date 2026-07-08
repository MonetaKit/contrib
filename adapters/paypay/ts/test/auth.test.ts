import { test } from "node:test";
import assert from "node:assert/strict";
import { authHeader } from "../src/auth.ts";

// The same golden vectors as the Go adapter's auth_test.go, generated with
// PayPay's official node SDK — all three implementations must match byte for
// byte or real requests 401.
test("OPA-Auth golden: POST with body", async () => {
  const body = JSON.stringify({
    merchantPaymentId: "sub_1-1700000000-0",
    userAuthorizationId: "ua_1",
    amount: { amount: 1200, currency: "JPY" },
    requestedAt: 1700000000,
  });
  assert.equal(
    body,
    `{"merchantPaymentId":"sub_1-1700000000-0","userAuthorizationId":"ua_1","amount":{"amount":1200,"currency":"JPY"},"requestedAt":1700000000}`,
    "serialized body drifted from the signed golden bytes",
  );
  assert.equal(
    await authHeader("test_client", "test_secret", "POST", "/v1/subscription/payments", body, "fixed-nonce", 1700000000),
    "hmac OPA-Auth:test_client:zMTNgAd6+TmrBk5R7MFnktNZvqAyPK7wRrs463lL5Nw=:fixed-nonce:1700000000:Uk/Q2Rb9c83A5fGGq/p+pA==",
  );
});

test("OPA-Auth golden: GET without body", async () => {
  assert.equal(
    await authHeader("test_client", "test_secret", "GET", "/v2/payments/sub_1-1700000000-0", null, "fixed-nonce", 1700000000),
    "hmac OPA-Auth:test_client:APmTrlUINlSWb9ZwQ9wYmaGp+fsajNIZH9BJ+1AO+MY=:fixed-nonce:1700000000:empty",
  );
});
