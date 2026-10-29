// Purpose: pin the REST design hygiene rules, bounded API reads, CLI reports and secret-scoped workflow.
// Depends on: node:test, synthetic recorded JSON, scripts/design/figma-lint.mjs; no real Figma credentials or network.
// Used by: scripts/dev/test-node.sh in the Foundation CI gate.
// Invariants: fixtures contain synthetic text only; all external reads are injected; existing gate assertions stay untouched.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

// Keep RED on an assertion naming this card, rather than on an unrelated setup error.
let lint;
try { lint = await import("../../scripts/design/figma-lint.mjs"); }
catch (error) { if (error.code !== "ERR_MODULE_NOT_FOUND") throw error; }
test("FIGMA-DESIGN-LINT implementation module exists", () => {
  assert.ok(lint, "scripts/design/figma-lint.mjs must implement the design hygiene checks");
});
const check = (name, fn) => test(name, { skip: !lint }, fn);
const fixtures = JSON.parse(readFileSync(new URL("./fixtures/frames.json", import.meta.url), "utf8"));
const copy = (value) => structuredClone(value);
const jsonResponse = (value) => new Response(JSON.stringify(value), { status: 200 });
const page = (children) => ({ id: "4:8829", type: "CANVAS", children });
const nodes = (documents) => ({ nodes: Object.fromEntries(documents.map((document) => [document.id, { document }])) });

check("BANNED is verbatim from the supplied authority (no additions or removals)", () => {
  const expected = [
    /\bMOCK\b/, /\bUNKNOWN\b/i, /\bqueued\b/i, /\bJSON\b/, /\bMiB\b/, /\bERASE\b/,
    /設計原型|設計示範|示範原型|原型示範|原型只|此畫框|回應示範|演練[:：]|合成資料|未連接服務/,
    /伺服器/, /工作階段/, /快照/, /≠/, /輸入框為有限示範/,
  ];
  assert.deepEqual(lint.BANNED.map(String), expected.map(String));
  for (const text of ["MOCK", "unknown", "Queued", "JSON", "MiB", "ERASE", "設計原型", "演練：", "伺服器", "工作階段", "快照", "≠", "輸入框為有限示範"]) {
    const frame = copy(fixtures.banned); frame.children[0].characters = text;
    assert.equal(lint.checkFrame(frame).filter((hit) => hit.rule === "COPY").length, 1, text);
  }
});
check("visible banned text reports its owning frame and text node", () => {
  assert.deepEqual(lint.checkFrame(fixtures.banned), [{ frameId: "1:1", nodeId: "1:2", rule: "COPY", text: "MOCK 訂單" }]);
});
check("exact 示範資料 is allowed; an appended banned term is checked", () => {
  assert.deepEqual(lint.checkFrame(fixtures.allowed), []);
  const frame = copy(fixtures.allowed); frame.children[0].characters += " MOCK";
  assert.equal(lint.checkFrame(frame)[0].rule, "COPY");
});
check("hidden nodes and ancestors suppress all descendant hits", () => {
  assert.deepEqual(lint.checkFrame(fixtures.hiddenAncestor), []);
  assert.deepEqual(lint.checkFrame({ ...fixtures.banned, visible: false }), []);
});
check("only matching top-level frames are selected; superseded frames are skipped", () => {
  const container = { id: "7:1", type: "SECTION", name: "other", children: [fixtures.banned] };
  assert.deepEqual(lint.selectFrames(page([fixtures.allowed, fixtures.superseded, container])), [fixtures.allowed]);
  assert.deepEqual(lint.checkFrame(fixtures.superseded), []);
});
check("1132px render overflow is WIDTH; clipping and ±1px tolerance are respected", () => {
  assert.equal(lint.checkFrame(fixtures.overflow)[0].rule, "WIDTH");
  assert.deepEqual(lint.checkFrame(fixtures.clipped), []);
  for (const width of [389, 390, 391]) {
    assert.deepEqual(lint.checkFrame({ ...fixtures.overflow, absoluteRenderBounds: { width } }), []);
  }
  assert.equal(lint.checkFrame({ ...fixtures.overflow, absoluteRenderBounds: { width: 388.9 } })[0].rule, "WIDTH");
});
check("visible stale and fix- descendants are flagged even without banned text", () => {
  const frame = copy(fixtures.allowed);
  frame.children = [{ id: "8:1", type: "GROUP", children: [
    { id: "8:2", type: "RECTANGLE", name: "已被取代 · old" },
    { id: "8:3", type: "RECTANGLE", name: "fix-overflow" },
    { id: "8:4", type: "RECTANGLE", name: "fix-hidden", visible: false },
  ] }];
  assert.deepEqual(lint.checkFrame(frame).map((hit) => hit.nodeId), ["8:2", "8:3"]);
  frame.children[0].visible = false;
  assert.deepEqual(lint.checkFrame(frame), []);
});

// Tests execute serially in this file; always restore the synthetic environment token.
async function withToken(fn) {
  const previous = process.env.FIGMA_TOKEN;
  process.env.FIGMA_TOKEN = "synthetic-figma-credential";
  try { return await fn(process.env.FIGMA_TOKEN); }
  finally { if (previous === undefined) delete process.env.FIGMA_TOKEN; else process.env.FIGMA_TOKEN = previous; }
}
check("REST discovers top-level frames, batches ≤50 ids and keeps token out of URLs", () => withToken(async (token) => {
  const frames = Array.from({ length: 101 }, (_, i) => ({ ...fixtures.allowed, id: `10:${i}` }));
  const requests = [];
  const report = await lint.lintFile({ fetchImpl: async (url, options) => {
    const parsed = new URL(url); requests.push(parsed);
    assert.equal(parsed.origin, "https://api.figma.com");
    assert.equal(parsed.pathname, "/v1/files/iJ9lDdFIXgTlKkt5Tvrc0X/nodes");
    assert.equal(options.headers["X-Figma-Token"], token);
    assert.equal(String(url).includes(token), false);
    assert.equal(options.method, "GET");
    assert.equal(options.redirect, "error", "never forward credentials through redirects");
    const ids = parsed.searchParams.get("ids").split(","); assert.ok(ids.length <= 50);
    if (ids[0] === "4:8829") return jsonResponse(nodes([page([...frames, fixtures.superseded])]));
    return jsonResponse(nodes(ids.map((id) => frames.find((frame) => frame.id === id))));
  } });
  assert.equal(requests.length, 4);
  assert.equal(report.frames, 101);
  assert.deepEqual(report.hits, []);
}));
check("429 and 5xx back off, honour Retry-After, and stop at three total tries", () => withToken(async () => {
  const waits = []; let calls = 0;
  const report = await lint.lintFile({ sleep: async (ms) => waits.push(ms), fetchImpl: async () => {
    calls++;
    if (calls === 1) return new Response(null, { status: 429, headers: { "Retry-After": "3" } });
    if (calls === 2) return new Response(null, { status: 503 });
    return jsonResponse(nodes([page([fixtures.allowed]), fixtures.allowed]));
  } });
  assert.deepEqual(report.hits, []);
  assert.equal(waits.length, 2); assert.ok(waits[0] >= 3000); assert.ok(waits[1] >= 1000);
  calls = 0;
  await assert.rejects(lint.lintFile({ sleep: async () => {}, fetchImpl: async () => {
    calls++; return new Response(null, { status: 503 });
  } }), /503/);
  assert.equal(calls, 3);
}));
check("auth failures are not retried and never include the response body or credential", () => withToken(async (token) => {
  let calls = 0;
  await assert.rejects(lint.lintFile({ fetchImpl: async () => {
    calls++; return new Response(token, { status: 403 });
  } }), (error) => /403/.test(error.message) && !error.message.includes(token));
  assert.equal(calls, 1);
}));
check("missing page, missing fetched frame, and zero matching frames fail closed", () => withToken(async () => {
  for (const response of [{ nodes: {} }, nodes([page([])])]) {
    await assert.rejects(lint.lintFile({ fetchImpl: async () => jsonResponse(response) }));
  }
  let calls = 0;
  await assert.rejects(lint.lintFile({ fetchImpl: async () => jsonResponse(++calls === 1 ? nodes([page([fixtures.allowed])]) : { nodes: { "2:1": null } }) }));
}));
check("CLI reports hits, summary and JSON, supports overrides, and returns 0/1/2", () => withToken(async () => {
  const dir = await mkdtemp(join(tmpdir(), "figma-lint-"));
  const lines = []; const originalLog = console.log, originalError = console.error;
  console.log = (...args) => lines.push(args.join(" ")); console.error = (...args) => lines.push(args.join(" "));
  try {
    for (const [frame, expected] of [[fixtures.banned, 1], [fixtures.allowed, 0]]) {
      const reportPath = join(dir, `report-${expected}.json`);
      const rc = await lint.main(["--file-key", "syntheticFile", "--page-id", "9:1", "--report", reportPath], {
        fetchImpl: async (url) => {
          const parsed = new URL(url); assert.equal(parsed.pathname, "/v1/files/syntheticFile/nodes");
          return jsonResponse(nodes([{ ...page([frame]), id: "9:1" }, frame]));
        },
      });
      assert.equal(rc, expected);
      const report = JSON.parse(await readFile(reportPath, "utf8"));
      assert.equal(report.hits.length, expected);
    }
    assert.ok(lines.some((line) => /^HIT 1:1 1:2 COPY :: MOCK 訂單$/.test(line)));
    assert.ok(lines.some((line) => /figma-lint:.*frames=1.*hits=1/.test(line)));
    const errorReport = join(dir, "error.json");
    assert.equal(await lint.main(["--report", errorReport], { fetchImpl: async () => new Response(null, { status: 401 }) }), 2);
    assert.equal(JSON.parse(await readFile(errorReport, "utf8")).status, "error");
    delete process.env.FIGMA_TOKEN;
    assert.equal(await lint.main(["--report", join(dir, "missing-token.json")], { fetchImpl: async () => { throw new Error("must not fetch"); } }), 2);
  } finally { console.log = originalLog; console.error = originalError; await rm(dir, { recursive: true, force: true }); }
}));
check("workflow is dispatch/nightly only, scoped read-only, bounded and always uploads the report", () => {
  const workflow = readFileSync(new URL("../../.github/workflows/figma-design-lint.yml", import.meta.url), "utf8");
  assert.match(workflow, /workflow_dispatch:/);
  assert.match(workflow, /cron: ['"]0 1 \* \* \*['"]/);
  assert.doesNotMatch(workflow, /^\s*(?:push|pull_request|pull_request_target):/m);
  assert.match(workflow, /environment: FIGMA_TOKEN/);
  assert.match(workflow, /contents: read/);
  assert.doesNotMatch(workflow, /:\s*write\b/);
  assert.match(workflow, /timeout-minutes: 10/);
  assert.match(workflow, /concurrency:\s*\n\s+group:/);
  assert.match(workflow, /FIGMA_TOKEN: \$\{\{ secrets\.FIGMA_TOKEN \}\}/);
  assert.match(workflow, /node scripts\/design\/figma-lint\.mjs/);
  assert.match(workflow, /if: always\(\)/);
  assert.match(workflow, /actions\/upload-artifact@[a-f0-9]{40}/);
});
