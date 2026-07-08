import { test, before, after, afterEach } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { createClient, type ChargeResult } from "../src/client.ts";

// Replays the SAME vectors/gateway.json as the Go adapter — the two
// implementations cannot drift on charge classification or request shape.
const doc = JSON.parse(
  readFileSync(new URL("../../vectors/gateway.json", import.meta.url), "utf8"),
) as {
  vectors: Array<{
    name: string;
    token: string;
    amount: number;
    currency: string;
    idempotencyKey: string;
    chargeDescription?: string;
    responses: Record<string, { status: number; body: unknown }>;
    expect: {
      result?: { status: string; providerRef: string };
      action?: { type: string; value: string };
      error?: boolean;
      noChargeCall?: boolean;
      chargeRequest?: Record<string, unknown>;
    };
  }>;
};

const BASE = "http://paypay.test";
const server = setupServer();
before(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
after(() => server.close());

for (const v of doc.vectors) {
  test(v.name, async () => {
    let calls = 0;
    let gotPath = "";
    let gotBody: Record<string, any> = {};
    let gotAuth = "";
    let gotMerchant = "";
    server.use(
      ...Object.entries(v.responses).map(([path, resp]) =>
        http.all(`${BASE}${path}`, async ({ request }) => {
          calls++;
          if (request.method === "POST") {
            gotPath = new URL(request.url).pathname;
            gotBody = (await request.json()) as Record<string, any>;
            gotAuth = request.headers.get("Authorization") ?? "";
            gotMerchant = request.headers.get("X-ASSUME-MERCHANT") ?? "";
          }
          return HttpResponse.json(resp.body as any, { status: resp.status });
        }),
      ),
    );

    const client = createClient({
      apiKey: "test_key",
      apiSecret: "test_secret",
      merchantId: "test_merchant",
      baseUrl: BASE,
      now: () => 1700000000,
      nonce: () => "fixed-nonce",
    });

    let res: ChargeResult | undefined;
    let err: unknown;
    try {
      res = await client.charge(
        v.token,
        { amount: v.amount, currency: v.currency },
        { idempotencyKey: v.idempotencyKey, description: v.chargeDescription },
      );
    } catch (e) {
      err = e;
    }

    if (v.expect.noChargeCall) assert.equal(calls, 0, "expected no API call");
    if (v.expect.error) {
      assert.ok(err, `want error, got result ${JSON.stringify(res)}`);
      return;
    }
    assert.ifError(err);
    assert.equal(res!.status, v.expect.result!.status);
    assert.equal(res!.providerRef, v.expect.result!.providerRef);
    if (v.expect.action) {
      assert.deepEqual(res!.action, v.expect.action);
    }
    for (const [key, want] of Object.entries(v.expect.chargeRequest ?? {})) {
      const got =
        key === "path" ? gotPath :
        key === "amount" || key === "currency" ? gotBody.amount?.[key] :
        gotBody[key];
      assert.deepEqual(got, want, `chargeRequest[${key}]`);
    }
    if (calls > 0) {
      assert.match(gotAuth, /^hmac OPA-Auth:test_key:/);
      assert.equal(gotMerchant, "test_merchant");
    }
  });
}

test("non-JSON response body -> paypay-prefixed error with status (Go parity)", async () => {
  server.use(
    http.post(`${BASE}/v1/subscription/payments`, () =>
      new HttpResponse("<html>502 Bad Gateway</html>", { status: 502, headers: { "Content-Type": "text/html" } }),
    ),
  );
  const client = createClient({ apiKey: "k", apiSecret: "s", baseUrl: BASE, now: () => 1700000000, nonce: () => "n" });
  await assert.rejects(
    client.charge("ua_1", { amount: 100, currency: "jpy" }, { idempotencyKey: "k1" }),
    /paypay: POST \/v1\/subscription\/payments -> 502: non-OPA response body/,
  );
});
