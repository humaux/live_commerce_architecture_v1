// Exercises the storefront gate's fail-closed WebKit pageerror boundary without network traffic.
import assert from "node:assert/strict";
import test from "node:test";
import { spawnSync } from "node:child_process";

process.env.LC_BROWSER_ENGINE = "webkit";
const { isOwnRscPageError } = await import("./c7-rsc-pageerror.mjs");
const origin = "https://shop.example";
const signature = (message, name = "Fetch API cannot load https") => ({ name, message });
const good = signature("/shop.example/en/search?_rsc=abc123 due to access control checks.");
const errors = [
  ["uncaught TypeError", { name: "TypeError", message: "Unexpected application exception" }],
  ["wrong name", signature(good.message, "Error")],
  ["foreign origin", signature("/evil.example/en/search?_rsc=abc123 due to access control checks.")],
  ["non-RSC API", signature("/shop.example/api/buyer/contact due to access control checks.")],
  ["API with RSC query", signature("/shop.example/api/buyer/contact?_rsc=abc123 due to access control checks.")],
  ["unmatched error", signature("/shop.example/en/search?_rsc=abc123 different error.")],
  ["other query data", signature("/shop.example/en/search?other=value&_rsc=abc123 due to access control checks.")],
  ["different port", signature("/shop.example:8443/en/search?_rsc=abc123 due to access control checks.")],
];
test("recognizes only the exact same-origin RSC WebKit signature", () => assert.equal(isOwnRscPageError(good, origin), true));
for (const [name, error] of errors) test(`${name} remains an unexpected error and fails the gate`, () => {
  const unexpected = [];
  if (!isOwnRscPageError(error, origin)) unexpected.push(error);
  assert.throws(() => assert.deepEqual(unexpected, []));
});
test("Chromium never classifies the WebKit signature", () => {
  const run = spawnSync(process.execPath, ["--input-type=module", "-e", `import { isOwnRscPageError } from './apps/storefront/tests/c7-rsc-pageerror.mjs'; if (isOwnRscPageError(${JSON.stringify(good)}, ${JSON.stringify(origin)})) process.exit(1);`], { cwd: process.cwd(), env: { ...process.env, LC_BROWSER_ENGINE: "chromium" }, encoding: "utf8" });
  assert.equal(run.status, 0, run.stderr);
});
