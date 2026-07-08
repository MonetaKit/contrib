import { test } from "node:test";
import assert from "node:assert/strict";
import { createHmac } from "node:crypto";
import { verifyResponseToken } from "../src/vault.ts";

// Signs an HS256 JWT the way PayPay signs responseTokens (key = the
// Base64-DECODED API secret).
const KEY = Buffer.from("0123456789abcdef0123456789abcdef");
const API_SECRET = KEY.toString("base64");

function signJWT(key: Buffer, header: object, claims: object): string {
  const b64url = (o: object) => Buffer.from(JSON.stringify(o)).toString("base64url");
  const signing = `${b64url(header)}.${b64url(claims)}`;
  const sig = createHmac("sha256", key).update(signing).digest("base64url");
  return `${signing}.${sig}`;
}

const HS256 = { alg: "HS256", typ: "JWT" };
const claims = (overrides: Record<string, unknown> = {}) => ({
  result: "succeeded",
  userAuthorizationId: "ua_9",
  aud: "client_1",
  iss: "paypay.ne.jp",
  exp: 1700003600,
  ...overrides,
});
const OPTS = { apiKey: "client_1", apiSecret: API_SECRET, now: () => 1700000000 };

test("valid token yields the userAuthorizationId", async () => {
  assert.equal(await verifyResponseToken(signJWT(KEY, HS256, claims()), OPTS), "ua_9");
});

const valid = signJWT(KEY, HS256, claims());
const forged = Buffer.from(JSON.stringify(claims({ userAuthorizationId: "ua_ATTACKER" }))).toString("base64url");
const [h, , s] = valid.split(".");

const noExp = (() => {
  const { exp: _drop, ...rest } = claims();
  return signJWT(KEY, HS256, rest);
})();

const rejects: Record<string, string> = {
  "missing exp": noExp,
  "undecodable signature segment": valid.split(".").slice(0, 2).join(".") + ".!!!not-base64url!!!",
  "tampered claims": `${h}.${forged}.${s}`,
  "alg none": signJWT(KEY, { alg: "none" }, claims()),
  "wrong signing key": signJWT(Buffer.from("wrong-key"), HS256, claims()),
  expired: signJWT(KEY, HS256, claims({ exp: 1600000000 })),
  "audience mismatch": signJWT(KEY, HS256, claims({ aud: "someone_else" })),
  "wrong issuer": signJWT(KEY, HS256, claims({ iss: "evil.example" })),
  "user declined": signJWT(KEY, HS256, claims({ result: "declined" })),
  "not a JWT": "not-a-jwt",
};

for (const [name, token] of Object.entries(rejects)) {
  test(`rejects: ${name}`, async () => {
    await assert.rejects(verifyResponseToken(token, OPTS));
  });
}
