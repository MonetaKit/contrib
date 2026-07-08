// Account-link responseToken verification — the TS twin of the Go adapter's
// Vault. The redirect from PayPay's account-link flow carries a responseToken
// JWT (HS256, keyed with the Base64-DECODED API secret); verifying it yields
// the userAuthorizationId that charge() accepts as its token.

const te = new TextEncoder();

function b64urlDecode(s: string): Uint8Array {
  const padded = s.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(s.length / 4) * 4, "=");
  const bin = atob(padded);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function decodeSecret(secret: string): Uint8Array {
  const padded = secret.padEnd(Math.ceil(secret.length / 4) * 4, "=");
  const bin = atob(padded); // throws on invalid base64
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

export interface VerifyOptions {
  apiKey: string;
  apiSecret: string;
  /** epoch seconds; injectable for tests */
  now?: () => number;
}

/**
 * Verifies a responseToken (alg pinned to HS256, constant-time MAC via Web
 * Crypto, iss/aud/exp/result enforced) and returns the userAuthorizationId.
 */
export async function verifyResponseToken(token: string, opts: VerifyOptions): Promise<string> {
  const parts = token.split(".");
  if (parts.length !== 3) throw new Error("paypay: responseToken is not a JWT");

  let header: { alg?: string };
  try {
    header = JSON.parse(new TextDecoder().decode(b64urlDecode(parts[0])));
  } catch {
    throw new Error("paypay: responseToken header is not valid");
  }
  if (header.alg !== "HS256") throw new Error("paypay: responseToken alg must be HS256");

  let keyBytes: Uint8Array;
  try {
    keyBytes = decodeSecret(opts.apiSecret);
  } catch {
    throw new Error("paypay: API secret is not valid base64 (required to verify responseToken)");
  }
  const key = await crypto.subtle.importKey(
    "raw", keyBytes as BufferSource, { name: "HMAC", hash: "SHA-256" }, false, ["verify"],
  );
  let sigBytes: Uint8Array;
  try {
    sigBytes = b64urlDecode(parts[2]);
  } catch {
    // An undecodable signature IS a signature mismatch (Go parity).
    throw new Error("paypay: responseToken signature mismatch");
  }
  const ok = await crypto.subtle.verify(
    "HMAC", key, sigBytes as BufferSource, te.encode(`${parts[0]}.${parts[1]}`),
  );
  if (!ok) throw new Error("paypay: responseToken signature mismatch");

  let claims: {
    result?: string;
    userAuthorizationId?: string;
    aud?: string;
    iss?: string;
    exp?: number;
  };
  try {
    claims = JSON.parse(new TextDecoder().decode(b64urlDecode(parts[1])));
  } catch {
    throw new Error("paypay: responseToken claims are not valid");
  }
  if (claims.iss !== "paypay.ne.jp") throw new Error(`paypay: responseToken issuer ${JSON.stringify(claims.iss)} is not paypay.ne.jp`);
  if (claims.aud !== opts.apiKey) throw new Error("paypay: responseToken audience mismatch");
  const now = opts.now ?? (() => Math.floor(Date.now() / 1000));
  if (!claims.exp) throw new Error("paypay: responseToken has no exp claim — non-expiring link tokens are not accepted");
  if (now() >= claims.exp) throw new Error("paypay: responseToken expired");
  if (claims.result !== "succeeded") throw new Error(`paypay: account link not granted (result ${JSON.stringify(claims.result)})`);
  if (!claims.userAuthorizationId) throw new Error("paypay: responseToken carries no userAuthorizationId");
  return claims.userAuthorizationId;
}
