import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { paymentReturn } from "../lib/payment-return.ts";

test("BPU03 neutral GET and POST return ignore every callback field and cookie", async () => {
  const request = new Request(
    "https://pay.example.test/payment/return?paid=true&store=https://evil.test",
    {
      method: "POST",
      headers: {
        Cookie: "buyer=secret",
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: "status=SUCCESS&return_url=https://evil.test&EncryptInfo=private-canary",
    },
  );
  const response = paymentReturn(request);
  const html = await response.text();
  assert.equal(html, await paymentReturn().text());
  assert.equal(request.bodyUsed, false);
  assert.equal(response.status, 200);
  for (const name of ["Location", "Set-Cookie"])
    assert.equal(response.headers.get(name), null);
  assert.equal(response.headers.get("Cache-Control"), "no-store");
  assert.equal(response.headers.get("Referrer-Policy"), "no-referrer");
  for (const secret of [
    "evil.test",
    "private-canary",
    "buyer=secret",
    "SUCCESS",
  ])
    assert.ok(!html.includes(secret));
  assert.ok(!/<(?:script|form|input|a)\b/i.test(html));
  for (const locale of ["en", "zh-CN", "zh-TW"])
    assert.ok(html.includes(`lang="${locale}"`));
  assert.ok(html.includes("does not confirm payment"));
  assert.ok(html.includes("seed baaadec1"));
  const style = html.match(/<style>(.*?)<\/style>/s)?.[1];
  assert.ok(style);
  const policy = response.headers.get("Content-Security-Policy");
  assert.ok(
    policy.includes(
      `'sha256-${createHash("sha256").update(style).digest("base64")}'`,
    ),
  );
  for (const directive of [
    "default-src 'none'",
    "base-uri 'none'",
    "frame-ancestors 'none'",
    "form-action 'none'",
  ])
    assert.ok(policy.includes(directive));
});
