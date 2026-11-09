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
  ["preview query", signature("/shop.example/en/search?preview=fixture&_rsc=abc123 due to access control checks.")],
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

test("actual storefront gate admits every pageerror into the zero-error assertion", async () => {
  const { readFile } = await import("node:fs/promises");
  const vm = await import("node:vm");
  const source = await readFile("tests/storefront/shop-real-gate.mjs", "utf8");
  const begin = source.indexOf('  p.on("pageerror", (e) => {');
  const end = source.indexOf("  if (rscDiagnostic) {", begin);
  assert.ok(begin >= 0 && end > begin, "actual pageerror listener must be present");
  let listener;
  const problems = [];
  vm.runInNewContext(source.slice(begin, end), {
    p: { on: (event, callback) => { assert.equal(event, "pageerror"); listener = callback; } },
    problems, safeObserver: null, isOwnRscPageError, origin, recognizedRscPageErrors: [],
    observedUrl: raw => { const u = new URL(raw); return { host: u.host, path: u.pathname, rsc: true }; },
  });
  for (const [label, error] of [["matching same-origin RSC signature", good], ...errors]) {
    const before = problems.length;
    listener(error);
    assert.equal(problems.length, before + 1, `${label} was omitted from the actual gate`);
  }
  assert.throws(() => assert.deepEqual(problems, []), "unchanged zero-error assertion must fail");
});

test("actual storefront gate retains hydration warning failures", async () => {
  const { readFile } = await import("node:fs/promises");
  const vm = await import("node:vm");
  const source = await readFile("tests/storefront/shop-real-gate.mjs", "utf8");
  const line = source.split("\n").find(x => x.includes('p.on("console", (m) =>') && x.includes("problems.push"));
  assert.ok(line, "actual hydration console listener must be present");
  let listener; const problems = [];
  vm.runInNewContext(line, { p: { on: (_, cb) => { listener = cb; } }, problems, safeObserver: null });
  listener({ type: () => "error", text: () => "Warning: hydration mismatch" });
  assert.equal(problems.length, 1);
  assert.throws(() => assert.deepEqual(problems, []));
});
