// OPA-Auth HMAC request signing — the TS twin of the Go adapter's auth.go,
// pinned to the same golden vectors (generated with PayPay's official node
// SDK). Edge-native: HMAC via Web Crypto; MD5 (protocol checksum) in ./md5.ts.

import { md5 } from "./md5.ts";

const te = new TextEncoder();

export function bytesToBase64(bytes: Uint8Array): string {
  let bin = "";
  for (let i = 0; i < bytes.length; i += 0x8000) {
    bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  }
  return btoa(bin);
}

/**
 * Builds the `Authorization` header value:
 *
 *     digest = Base64(MD5(contentType || body))       // both "empty" when no body
 *     raw    = path\nmethod\nnonce\nepoch\ncontentType\ndigest
 *     sig    = Base64(HMAC-SHA256(raw, apiSecret))
 *     hmac OPA-Auth:{apiKey}:{sig}:{nonce}:{epoch}:{digest}
 *
 * `path` is the URL path only (no query, no host); `body` must be the exact
 * string sent on the wire.
 */
export async function authHeader(
  apiKey: string,
  apiSecret: string,
  method: string,
  path: string,
  body: string | null,
  nonce: string,
  epoch: number,
): Promise<string> {
  let contentType = "empty";
  let digest = "empty";
  if (body) {
    contentType = "application/json";
    const ct = te.encode(contentType);
    const b = te.encode(body);
    const data = new Uint8Array(ct.length + b.length);
    data.set(ct);
    data.set(b, ct.length);
    digest = bytesToBase64(md5(data));
  }
  const raw = [path, method, nonce, String(epoch), contentType, digest].join("\n");
  const key = await crypto.subtle.importKey(
    "raw", te.encode(apiSecret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"],
  );
  const sig = bytesToBase64(new Uint8Array(await crypto.subtle.sign("HMAC", key, te.encode(raw))));
  return `hmac OPA-Auth:${apiKey}:${sig}:${nonce}:${epoch}:${digest}`;
}
