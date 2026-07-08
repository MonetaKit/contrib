import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseWebhook } from "../src/webhook.ts";

// Replays the SAME vectors/webhooks.json as the Go adapter.
const doc = JSON.parse(
  readFileSync(new URL("../../vectors/webhooks.json", import.meta.url), "utf8"),
) as {
  vectors: Array<{ name: string; payload: unknown; expect: Record<string, unknown> }>;
};

for (const v of doc.vectors) {
  test(v.name, () => {
    const ev = parseWebhook(JSON.stringify(v.payload));
    // Strip undefined optionals so the comparison matches the vector's
    // omitted fields (mirrors Go's omitempty semantics).
    assert.deepEqual(JSON.parse(JSON.stringify(ev)), v.expect);
  });
}

test("fails closed when a signature or secret is supplied", () => {
  const payload = `{"notification_type":"Transaction","state":"COMPLETED","order_id":"x","order_amount":1}`;
  assert.throws(() => parseWebhook(payload, "sig", undefined));
  assert.throws(() => parseWebhook(payload, undefined, "whsec"));
});
