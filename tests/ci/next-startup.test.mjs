// Purpose: real loopback/socket and owned-child regression for the Next harness port handoff race, plus the #116 static guard over every migrated harness.
// Depends on: tests/helpers/next-startup.mjs, Node child_process/net/http/fs; synthetic bind-server fixture only.
// Used by: scripts/dev/test-node.sh and GATE-PORT acceptance. Never uses a customer endpoint or production process.
import assert from "node:assert/strict";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import http from "node:http";
import { createWriteStream } from "node:fs";
import { mkdtemp, rm, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript-api";
import vm from "node:vm";
import { allocateNextPort, nextAttemptLog, startNextWithPortRetry, connectFixtureAdmin } from "../helpers/next-startup.mjs";

const bindProgram = `
const http = require('node:http');
const server = http.createServer((_, response) => response.end('ready'));
server.on('error', error => { console.error(error.code); process.exit(2); });
server.listen(Number(process.argv[1]), '127.0.0.1', () => process.send({ready:true}));
`;
const running = (child) => child.exitCode === null && child.signalCode === null;

async function fixture(run) {
  const dir = await mkdtemp(path.join(tmpdir(), "lc-next-bind-"));
  const children = [], logs = [], occupied = [], ports = [];
  async function occupy(port) {
    const server = http.createServer((_, response) => { response.writeHead(503); response.end(); });
    server.listen(port, "127.0.0.1"); await once(server, "listening"); occupied.push(server);
  }
  async function start({ port, attempt, track }, program = bindProgram) {
    ports.push(port);
    const file = nextAttemptLog(path.join(dir, "next.log"), attempt);
    const log = createWriteStream(file, { flags: "wx", mode: 0o600 });
    logs.push(log); await once(log, "open");
    const child = spawn(process.execPath, ["-e", program, String(port)], { env: {}, stdio: ["ignore", log, log, "ipc"] });
    children.push(child); track(child, log);
    return new Promise((resolve, reject) => {
      child.once("message", () => resolve({ port, child, file }));
      child.once("exit", () => reject(new Error("owned Next exited before readiness")));
      child.once("error", reject);
    });
  }
  try { return await run({ dir, children, logs, ports, occupy, start }); }
  finally {
    for (const child of children) if (running(child)) { const exit = once(child, "exit"); child.kill("SIGTERM"); await exit; }
    for (const log of logs) if (!log.closed) { const close = once(log, "close"); log.end(); await close; }
    for (const server of occupied) await new Promise((resolve) => server.close(resolve));
    await rm(dir, { recursive: true, force: true });
  }
}

test("an automatic port stolen after reservation retries and reaches the owned listener", { timeout: 10000 }, async () => fixture(async ({ dir, ports, occupy, start }) => {
  let allocations = 0;
  const result = await startNextWithPortRetry({ allocatePort: async () => {
    const port = await allocateNextPort();
    if (++allocations === 1) await occupy(port); // real TOCTOU: another listener holds the helper's selected port before spawn
    return port;
  } }, start);
  assert.equal(allocations, 2);
  assert.equal(ports.length, 2);
  assert.notEqual(ports[0], ports[1]);
  assert.match(await readFile(path.join(dir, "next.log"), "utf8"), /EADDRINUSE/);
  assert.match(result.file, /attempt-2$/);
  assert.equal(running(result.child), true);
  const response = await fetch(`http://127.0.0.1:${result.port}`, { signal: AbortSignal.timeout(1000) });
  assert.equal(await response.text(), "ready");
}));

test("the exact old publish-gate startNext fails on the same forced real port handoff", { timeout: 10000 }, async () => fixture(async ({ dir, children, logs, occupy }) => {
  const source = execFileSync("git", ["show", "9b738e0a:tests/storefront/storefront-publish-gate.mjs"], { encoding: "utf8" });
  const ast = ts.createSourceFile("old-gate.mjs", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const declaration = ast.statements.find((n) => ts.isFunctionDeclaration(n) && n.name?.text === "startNext");
  let selected;
  const oldStart = vm.runInNewContext(`${declaration.getText(ast)}; startNext`, {
    net: await import("node:net"), path, evidence: dir, root: "/synthetic-owned-next", logs, children: new Set(),
    process: { execPath: process.execPath, env: {} }, createWriteStream, once, running,
    wait: (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
    listen: async (server) => {
      server.listen(0, "127.0.0.1"); await once(server, "listening"); selected = server.address().port;
      const close = server.close.bind(server);
      server.close = (callback) => close(async (error) => { if (!error) await occupy(selected); callback(error); });
      return selected;
    },
    // Network/process edges only: original gate code and its 100×50ms readiness checks execute unchanged.
    spawn: (_executable, args, options) => {
      const child = spawn(process.execPath, ["-e", bindProgram, args.at(-1)], { env: {}, stdio: ["ignore", options.stdio[1], options.stdio[2], "ipc"] });
      children.push(child); return child;
    },
    relay: async (port) => ({ status: (await fetch(`http://127.0.0.1:${port}`, { signal: AbortSignal.timeout(1000) })).status }),
  });
  await assert.rejects(oldStart("storefront"), /exited before readiness/);
  assert.equal(children.length, 1);
  assert.match(await readFile(path.join(dir, "storefront.log"), "utf8"), /EADDRINUSE/);
}));

test("a caller-specified occupied port fails without allocation or retry", { timeout: 10000 }, async () => fixture(async ({ ports, occupy, start }) => {
  const port = await allocateNextPort(); await occupy(port);
  await assert.rejects(startNextWithPortRetry({ port, allocatePort: () => { throw new Error("must not allocate"); } }, start), /exited before readiness/);
  assert.deepEqual(ports, [port]);
}));

test("a non-bind startup crash is not retried", { timeout: 10000 }, async () => fixture(async ({ ports, start }) => {
  await assert.rejects(startNextWithPortRetry({}, (context) => start(context, "console.error('synthetic startup rejection');process.exit(3)")), /exited before readiness/);
  assert.equal(ports.length, 1);
}));

test("persistent automatic bind collisions stop after three total launches", { timeout: 10000 }, async () => fixture(async ({ ports, occupy, start }) => {
  let allocations = 0;
  await assert.rejects(startNextWithPortRetry({ allocatePort: async () => {
    const port = await allocateNextPort(); allocations++; await occupy(port); return port;
  } }, start), /exited before readiness/);
  assert.equal(allocations, 3);
  assert.equal(ports.length, 3);
}));

test("a readiness rejection while the child is alive preserves the original error and never retries", { timeout: 10000 }, async () => fixture(async ({ ports, start }) => {
  const error = new Error("synthetic readiness deadline");
  await assert.rejects(startNextWithPortRetry({}, async (context) => { await start(context); throw error; }), (caught) => caught === error);
  assert.equal(ports.length, 1);
}));

test("a failure before spawning is not retried", async () => {
  const error = new Error("synthetic log setup failure");
  let allocations = 0;
  await assert.rejects(startNextWithPortRetry({ allocatePort: async () => { allocations++; return allocateNextPort(); } }, async () => { throw error; }), (caught) => caught === error);
  assert.equal(allocations, 1);
});

test("ready admin registration uses the authenticated Go fixture seam once, with no restart on a lost acknowledgement", { timeout: 5000 }, async () => {
  const observed = [];
  let reply = 204;
  const server = http.createServer(async (request, response) => {
    const chunks = []; for await (const chunk of request) chunks.push(chunk);
    observed.push({ method: request.method, path: request.url, key: request.headers["x-gate-key"], body: JSON.parse(Buffer.concat(chunks)) });
    if (reply === "lost") { request.socket.destroy(); return; } // actual lost ACK after the fixture observed registration
    response.writeHead(reply); response.end();
  });
  server.listen(0, "127.0.0.1"); await once(server, "listening");
  try {
    const control = `http://127.0.0.1:${server.address().port}`;
    await connectFixtureAdmin(43121, control, "synthetic-control");
    assert.deepEqual(observed, [{ method: "POST", path: "/admin-upstream", key: "synthetic-control", body: { port: 43121 } }]);
    reply = 503;
    await assert.rejects(connectFixtureAdmin(43121, control, "synthetic-control"), /registration refused \(503\)/);
    assert.equal(observed.length, 2); // one request per call, not a retry of an acknowledged/uncertain startup
    reply = "lost";
    await assert.rejects(connectFixtureAdmin(43121, control, "synthetic-control"), /fetch failed/);
    assert.equal(observed.length, 3);
    await assert.rejects(connectFixtureAdmin(0, control, "synthetic-control"), /invalid fixture/);
    await assert.rejects(connectFixtureAdmin(43121, "http://example.invalid", "synthetic-control"), /invalid fixture/);
    assert.equal(observed.length, 3);
  } finally { await new Promise((resolve) => server.close(resolve)); }
});

test("the real publish-mode registry runs every public-admin ownership regression as well as the existing browser test", () => {
  const mode = execFileSync("bash", ["scripts/dev/test-local.sh", "--dry-run", "--browser-storefront-publish"], { encoding: "utf8" });
  const run = /-run '([^']+)'/.exec(mode);
  assert.ok(run, "actual registry must expose the Go invocation");
  const selected = new RegExp(run[1]);
  for (const name of ["TestBrowserStorefrontPublish", "TestBrowserAdminRelayOwnsExplicitOrigin", "TestBrowserAdminRelayPreservesOriginAndTLSHeaders", "TestBrowserAdminRelayPreservesPostCSRFAndRedirect", "TestBrowserAdminRelayHandoffHasNoFreePortWindow"])
    assert.equal(selected.test(name), true, `the CI mode omits ${name}`);
  assert.equal(selected.test("TestBrowserStoreDomains"), false);
});

test("the catalog-media and manual-order registries run their gate together with the freed-port handoff regression", () => {
  for (const [mode, main] of [["--browser-catalog-media", "TestBrowserCatalogMedia"], ["--browser-manual-order", "TestBrowserManualOrderLink"]]) {
    const dry = execFileSync("bash", ["scripts/dev/test-local.sh", "--dry-run", mode], { encoding: "utf8" });
    const run = /-run '([^']+)'/.exec(dry);
    assert.ok(run, `${mode}: actual registry must expose the Go invocation`);
    const selected = new RegExp(run[1]);
    assert.equal(selected.test(main), true, `${mode} omits ${main}`);
    assert.equal(selected.test("TestBrowserAdminRelayHandoffHasNoFreePortWindow"), true, `${mode} omits the freed-port handoff regression`);
    assert.equal(selected.test("TestBrowserStorefrontPublish"), false, `${mode} widened past its own gate`);
    assert.equal(selected.test("TestBrowserAdminRelayOwnsExplicitOrigin"), false, `${mode} widened past its own gate`);
  }
});

// Round-1 K3 P1: the last commit whose catalog-media/manual-order Go fixtures still freed the reserved public
// admin port and handed it to the gate as an explicit LC_*_ADMIN_PORT. The old source must fail the guard below.
// The red witness is kept inline: dd469ce8 is a topic-branch merge that a squash merge drops from r3/integration history,
// so `git show` of it would fail in any clone of the merged trunk (PR #37 review). These are the exact pre-fix lines.
const preFix = (env) => [
  'listener, err := net.Listen("tcp", "127.0.0.1:0")',
  "adminOrigin := browserFront(t, listener.Addr().String())",
  "_ = listener.Close()",
  `"${env}": adminPort,`,
].join("\n");

test("the converted catalog-media and manual-order Go fixtures no longer free the public admin port (static red/green)", async () => {
  // The guard the two fixtures must satisfy: no reserve-then-free handoff, an owned relay origin instead,
  // and the runner-only control seam the gate uses to attach its automatic-port admin.
  const noFreedPortHandoff = (file, source) => {
    assert.equal(/_ = listener\.Close\(\)/.test(source), false, `${file}: the public admin listener is freed before the gate binds it`);
    assert.equal(source.includes("net.Listen(\"tcp\", \"127.0.0.1:0\")"), false, `${file}: the gate still reserves a port it later frees`);
    assert.match(source, /newBrowserAdminRelay\(t\)/, `${file}: the public admin origin is not continuously owned by the relay`);
    assert.match(source, /admin\.connect\(w, r\)/, `${file}: the gate cannot register its automatic-port admin`);
  };
  for (const [file, portEnv] of [
    ["tests/foundation/browser_catalog_media_test.go", "LC_CM_ADMIN_PORT"],
    ["tests/foundation/browser_manual_order_link_test.go", "LC_LINK_ADMIN_PORT"],
  ]) {
    const before = preFix(portEnv);
    // RED: the pre-fix source is exactly the collision window (reserve -> browserFront -> Close -> explicit port).
    assert.ok(before.includes(portEnv) && before.includes("_ = listener.Close()"), `${file}: inline pre-fix witness lost the old handoff`);
    assert.throws(() => noFreedPortHandoff(file, before), /freed before the gate binds it/);
    // GREEN: the current source holds the listener and hands nothing over; the explicit port env is gone.
    const current = await readFile(file, "utf8");
    assert.equal(current.includes(portEnv), false, `${file}: the explicit-port handoff survives`);
    noFreedPortHandoff(file, current);
  }
});

test("connectFixtureAdmin keeps one bounded attempt with a CI-host-tolerant timeout", async () => {
  const source = await readFile("tests/helpers/next-startup.mjs", "utf8");
  const fn = /^export async function connectFixtureAdmin[\s\S]*?^\}/m.exec(source);
  assert.ok(fn, "connectFixtureAdmin missing from the shared helper");
  // K3 P2: 1 s was tight on a loaded CI host; 5 s bounds the single loopback control POST.
  assert.match(fn[0], /AbortSignal\.timeout\(5000\)/);
  // The no-blind-retry policy on a lost ACK stays: exactly one fetch, no loop.
  assert.equal(fn[0].match(/await fetch\(/g).length, 1);
  assert.equal(/\bfor\b|\bwhile\b/.test(fn[0]), false);
});

const GATE_BASELINE = "9b738e0af64ebd08c25b39f8081d46d4dc5674fb";
const gatePrinter = ts.createPrinter({ removeComments: true });

/**
 * AST parity of one gate adapter against GATE_BASELINE. Preserved classes: whole-file assertion calls,
 * environment member assignment expression statements (`env.HOSTNAME = "127.0.0.1"; env.PORT = String(port);`),
 * readiness status predicates wherever they sit in the adapter, readiness `for` loops, spawn calls and
 * env/childEnv/args/bin variable statements. K3 P2: the two middle classes used to escape the guard, so
 * dropping env.PORT or changing/moving the readiness predicate passed. Throws naming the violated class.
 * Returns false when the baseline file has no startNext/startStorefront adapter.
 */
function assertGateParity(file, before, currentSource) {
  const old = ts.createSourceFile(file, before, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const start = old.statements.find((n) => ts.isFunctionDeclaration(n) && ["startNext", "startStorefront"].includes(n.name?.text));
  if (!start) return false;
  const current = ts.createSourceFile(file, currentSource, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const text = (node, source) => gatePrinter.printNode(ts.EmitHint.Unspecified, node, source);
  const collect = (source, root, predicate) => {
    const out = []; const visit = (node) => { if (predicate(node, source)) out.push(text(node, source)); ts.forEachChild(node, visit); }; visit(root); return out;
  };
  const now = current.statements.find((n) => ts.isFunctionDeclaration(n) && n.name?.text === start.name.text);
  const assertions = (n, source) => ts.isCallExpression(n) && /^(assert|expect)(?:\.|\(|$)/.test(n.expression.getText(source));
  // Every baseline assertion must survive, in order (no weakening or removal); added assertions are allowed, since a
  // later fix may legitimately strengthen a gate (e.g. waiting for the regenerated link to render).
  const kept = collect(current, current, assertions), original = collect(old, old, assertions);
  let at = 0;
  for (const call of kept) if (at < original.length && call === original[at]) at++;
  assert.equal(at, original.length, `${file}: original gate assertions changed (missing: ${original[at]})`);
  const envAssignment = (n, source) => ts.isExpressionStatement(n) && /^(?:env|childEnv)\./.test(text(n, source));
  assert.deepEqual(collect(current, now, envAssignment), collect(old, start, envAssignment), `${file}: environment assignment statements changed`);
  const readiness = (n, source) => ts.isIfStatement(n) && /\bstatus\b/.test(n.expression.getText(source));
  assert.deepEqual(collect(current, now, readiness), collect(old, start, readiness), `${file}: readiness status predicate changed`);
  assert.deepEqual(collect(current, now, ts.isForStatement), collect(old, start, ts.isForStatement), `${file}: readiness/env loop changed`);
  const spawnCall = (n, source) => ts.isCallExpression(n) && n.expression.getText(source) === "spawn";
  assert.deepEqual(collect(current, now, spawnCall), collect(old, start, spawnCall), `${file}: spawned app/args/env changed`);
  const envStatement = (n, source) => ts.isVariableStatement(n) && n.declarationList.declarations.some((d) => ["env", "childEnv", "args", "bin"].includes(d.name.getText(source)));
  assert.deepEqual(collect(current, now, envStatement), collect(old, start, envStatement), `${file}: environment/command changed`);
  assert.ok(collect(current, now, (n, source) => ts.isCallExpression(n) && n.expression.getText(source) === "startNextWithPortRetry").length === 1, `${file}: shared helper missing`);
  assert.ok(!/net\.createServer\(|freePort\(|listen\((reserve|probe)\)/.test(now.getText(current)), `${file}: old reservation survives`);
  return true;
}

test("all eleven gate adapters preserve original assertions, readiness loops and predicates, spawn and environment statements", async () => {
  const files = execFileSync("git", ["ls-tree", "-r", "--name-only", GATE_BASELINE, "tests/storefront"], { encoding: "utf8" }).trim().split("\n").filter((f) => f.endsWith("-gate.mjs"));
  let checked = 0;
  for (const file of files) {
    const before = execFileSync("git", ["show", `${GATE_BASELINE}:${file}`], { encoding: "utf8" });
    if (assertGateParity(file, before, await readFile(file, "utf8"))) checked++;
  }
  assert.equal(checked, 11);
});

test("the parity guard rejects a dropped env.PORT and a changed or relocated readiness predicate (red mutations)", async () => {
  const file = "tests/storefront/catalog-media-gate.mjs";
  const before = execFileSync("git", ["show", `${GATE_BASELINE}:${file}`], { encoding: "utf8" });
  const current = await readFile(file, "utf8");
  assert.equal(assertGateParity(file, before, current), true); // the shipped adapter passes its own guard
  // RED 1 (K3 P2): dropping env.PORT used to slip through — no comparison covered expression statements.
  const dropped = current.replace('env.HOSTNAME = "127.0.0.1"; env.PORT = String(port);', 'env.HOSTNAME = "127.0.0.1";');
  assert.notEqual(dropped, current, "mutation target missing from the current gate");
  assert.throws(() => assertGateParity(file, before, dropped), /environment assignment statements changed/);
  // RED 2: weakening the readiness status predicate fails, named as such (before the loop comparison fires).
  const weakened = current.replace('if (response.status === (app === "admin" ? 401 : 200)) return port;', 'if (response.status === (app === "admin" ? 403 : 200)) return port;');
  assert.notEqual(weakened, current, "mutation target missing from the current gate");
  assert.throws(() => assertGateParity(file, before, weakened), /readiness status predicate changed/);
  // RED 3: the same predicate relocated OUTSIDE the readiness loop (loop text untouched) still fails.
  const relocated = current.replace('throw new Error(`owned ${app} readiness deadline`);', 'if (response.status < 500) return port;\n    throw new Error(`owned ${app} readiness deadline`);');
  assert.notEqual(relocated, current, "mutation target missing from the current gate");
  assert.throws(() => assertGateParity(file, before, relocated), /readiness status predicate changed/);
});

// #116 (review-111 P2-4): the browser harnesses that still reserved a loopback port, freed it and handed the number to a child
// (Next or `next dev`) could lose it to any parallel process in between (EADDRINUSE flakes, e.g. meta-health-ui in Gates
// 39332672115). The race itself is proven on real sockets above (Node: "an automatic port stolen after reservation ...") and in
// TestBrowserAdminRelayStartRetriesBindCollision (Go: relay.start and startPrivate). This guard binds every migrated site to
// those helpers. RED: each file at PORT_RACE_BASE (65715bc38, an r3/integration commit, so it survives a squash merge) still
// has the reserve-then-free handoff and must fail the guard. GREEN: the current file has none and routes its child through
// the shared helper.
const PORT_RACE_BASE = "65715bc38";
const goPortRaceSites = [
  "tests/foundation/browser_e2e_test.go", "tests/foundation/browser_live_tools_test.go",
  "tests/foundation/browser_live_claims_test.go", "tests/foundation/browser_product_media_v2_test.go",
  "tests/foundation/browser_studio_bff_test.go", "tests/foundation/browser_merchant_orders_bff_test.go",
];
const nodePortRaceSites = [
  "tests/ui/visual-audit.mjs", "tests/storefront/ads-attribution.mjs", "tests/storefront/legal-pages.mjs",
  "tests/storefront/ad-link.mjs", "tests/admin/ui-gallery-runner.mjs", "tests/admin/shell-runner.mjs",
  "tests/admin/platform-runner.mjs", "tests/admin/picklist.spec.ts",
  // review-131 P1: the storefront buyer harnesses' startNext also reserved, freed and handed the port to `next start --port`.
  "tests/storefront/ads-consent.mjs", "tests/storefront/collector-buyer.mjs", "tests/storefront/cvs-buyer.mjs",
  "tests/storefront/cvs-pap-only.mjs", "tests/storefront/home-cod-buyer.mjs", "tests/storefront/offline-buyer.mjs",
  "tests/storefront/privacy-buyer.mjs", "tests/storefront/refund-buyer.mjs", "tests/storefront/shipment-buyer.mjs",
  // found by the full re-sweep after review-131: the same reserve-then-free startNext in the click-sweep and Stripe browser storefronts.
  "tests/ui/click-sweep.mjs", "tests/storefront/stripe-browser.mjs",
];
function noPickThenRelease(file, source) {
  if (file.endsWith(".go")) {
    assert.equal(/net\.Listen\("tcp\w*",\s*"(127\.0\.0\.1|localhost|\[::1\])?:0"\)/.test(source), false, `${file}: a loopback port is still reserved and freed for a child`);
    assert.equal(/freeLoopbackPort\(/.test(source), false, `${file}: freeLoopbackPort still hands a freed port to a child`);
    assert.equal(/"PORT": \w|"--port", \w/.test(source), false, `${file}: a child still gets a caller-picked port`);
    assert.match(source, /[rR]elay\.start\(t, |startPrivate\(t, /, `${file}: the child is not launched through the relay (owned port + EADDRINUSE relaunch)`);
  } else {
    // `(probe|reserve).close(<callback>)` is the freeing call; a bare `.close()` (offline-buyer's `probe` browser context) is not a port.
    assert.equal(/\bcreateServer\(\)|freePort\(|listen\((reserve|probe)\)|(probe|reserve)\.close\([^)]/.test(source), false, `${file}: a loopback port is still reserved and freed for a child`);
    assert.match(source, /startNextWithPortRetry\(\{\}, /, `${file}: the child is not started through the shared automatic-port helper`);
    assert.match(source, /track\(\w+, \w+\)/, `${file}: the helper cannot see the child's exit and log (no EADDRINUSE proof)`);
  }
}

test("#116: every remaining pick-then-release browser harness routes its child through the shared port helpers (static red/green)", async () => {
  for (const file of [...goPortRaceSites, ...nodePortRaceSites]) {
    const before = execFileSync("git", ["show", `${PORT_RACE_BASE}:${file}`], { encoding: "utf8" });
    assert.throws(() => noPickThenRelease(file, before), /reserved and freed|freeLoopbackPort|caller-picked port/, `${file}: red witness lost the pre-fix handoff`);
    noPickThenRelease(file, await readFile(file, "utf8"));
  }
});

// TH-10 (#147): the Go half of the guard must see every spelling of a loopback pick-then-release and must not accept just any `.start(t, `.
test("TH-10: the Go port-race guard flags localhost:0, :0 and [::1]:0 and anchors the positive to the relay", () => {
  const routed = "relay := newBrowserAdminRelay(t)\nchild = relay.start(t, ctx, cmd)\n";
  noPickThenRelease("synthetic_test.go", routed); // control: a routed child passes
  noPickThenRelease("synthetic_test.go", "x.adminRelay.start(t, ctx, cmd)\n_, port := startPrivate(t, ctx, c)\n");
  for (const spelling of ['"127.0.0.1:0"', '"localhost:0"', '":0"', '"[::1]:0"'])
    assert.throws(() => noPickThenRelease("synthetic_test.go", `l, _ := net.Listen("tcp", ${spelling})\n${routed}`), /reserved and freed/, `${spelling} slipped past the guard`);
  assert.throws(() => noPickThenRelease("synthetic_test.go", "server.start(t, ctx, cmd)\n"), /not launched through the relay/, "a non-relay .start(t, ...) satisfied the positive");
});

// CMD-1 (#147): cmd/api binds LISTEN_ADDR itself (`:0` plus the api_ready line), so no Go test may hand a cmd/* binary a caller-picked
// port. Every LISTEN_ADDR assignment under tests/ must take the shared constant mrEphemeralAddr ("127.0.0.1:0"); the bound address is
// read back from the child's api_ready line (mrBoundAddr). RED: the files at CMD_BOUND_BASE (an r3/integration commit) still pick
// a port in advance and must fail; GREEN: the current files do not. COMMERCE_CLAIMS_CONSOLE_ADDR (claims-worker) is a different
// knob and the documented remaining exception (docs/delivery/GATES.md).
const CMD_BOUND_BASE = "b464ee4b4";
const listenAddrSites = [
  "tests/foundation/studio_process_test.go", "tests/foundation/local_recovery_test.go", "tests/foundation/studio_planning_only_test.go",
  "tests/foundation/meta_runtime_process_test.go", "tests/foundation/merchant_restart_replay_test.go", "tests/foundation/account_process_test.go",
  "tests/foundation/stripe_live_process_test.go", "tests/foundation/order_attempt_process_test.go",
];
function noPickedListenAddr(file, source) {
  const uses = [...source.matchAll(/LISTEN_ADDR/g)];
  assert.ok(uses.length > 0, `${file}: no LISTEN_ADDR use left, drop it from listenAddrSites`);
  for (const use of uses) {
    const rest = source.slice(use.index, use.index + 64);
    assert.match(rest, /^LISTEN_ADDR"(=" \+ |:\s*)mrEphemeralAddr\b/, `${file}: a cmd/* binary gets a caller-picked LISTEN_ADDR (${JSON.stringify(rest.slice(0, 40))}), use mrEphemeralAddr and read the api_ready line`);
  }
}
test("CMD-1: no Go test hands a cmd/* binary a pre-picked LISTEN_ADDR (static red/green)", async () => {
  for (const file of listenAddrSites) {
    const before = execFileSync("git", ["show", `${CMD_BOUND_BASE}:${file}`], { encoding: "utf8" });
    assert.throws(() => noPickedListenAddr(file, before), /caller-picked LISTEN_ADDR/, `${file}: red witness lost the pre-picked LISTEN_ADDR`);
    noPickedListenAddr(file, await readFile(file, "utf8"));
  }
  // Nothing else under tests/ may set LISTEN_ADDR either (a new site must be routed the same way).
  const all = execFileSync("git", ["ls-files", "tests"], { encoding: "utf8" }).split("\n").filter((f) => f.endsWith(".go"));
  for (const file of all) {
    const source = await readFile(file, "utf8");
    if (source.includes("LISTEN_ADDR")) noPickedListenAddr(file, source);
  }
  // Synthetic spellings: a pre-picked variable, a literal port and a quoted constant all fail; the shared constant passes.
  noPickedListenAddr("synthetic_test.go", '"LISTEN_ADDR=" + mrEphemeralAddr,\n"LISTEN_ADDR": mrEphemeralAddr');
  for (const bad of ['"LISTEN_ADDR=" + addr', '"LISTEN_ADDR": "127.0.0.1:" + port', '"LISTEN_ADDR=127.0.0.1:8080"', '"LISTEN_ADDR=" + "127.0.0.1:0"'])
    assert.throws(() => noPickedListenAddr("synthetic_test.go", bad), /caller-picked LISTEN_ADDR/, `${bad} slipped past the guard`);
});

// TH-9 (#147): the relay SIGKILLs the whole group the moment the leader exits, so a SIGINT-then-wait grace never reaches the
// next-server children. The fixtures must not pretend to grant one.
test("TH-9: studio-bff and orders-bff do not stop a relay-owned group with a SIGINT grace", async () => {
  for (const file of ["tests/foundation/browser_studio_bff_test.go", "tests/foundation/browser_merchant_orders_bff_test.go"]) {
    const source = await readFile(file, "utf8");
    assert.equal(/syscall\.SIGINT/.test(source), false, `${file}: SIGINT grace on a relay-owned group is dead code (the relay kills the group on leader exit)`);
    assert.match(source, /stopProcessGroup\(t, pgid\)/, `${file}: the fixture no longer sweeps its process group`);
  }
});

// TH-11 (#147): a signal-killed child has exitCode null and signalCode set; readiness must treat it as exited (tests/helpers/next-startup.mjs).
test("TH-11: visual-audit, ads-attribution, platform-runner and legal-pages readiness sees a signal-killed child", async () => {
  for (const file of ["tests/ui/visual-audit.mjs", "tests/storefront/ads-attribution.mjs", "tests/admin/platform-runner.mjs", "tests/storefront/legal-pages.mjs"]) {
    const source = await readFile(file, "utf8");
    assert.match(source, /\w+\.exitCode !== null \|\| \w+\.signalCode !== null/, `${file}: readiness checks exitCode only; a SIGKILLed child waits out the whole budget`);
  }
});

// TH-12 (#147): "wx" fails with EEXIST when LC_LEGAL_EVIDENCE_DIR is reused by a rerun.
test("TH-12: legal-pages opens its Next log so a rerun into the same evidence directory does not collide", async () => {
  const source = await readFile("tests/storefront/legal-pages.mjs", "utf8");
  assert.equal(/flags: "wx"/.test(source), false, "legal-pages opens its log with wx; a rerun fails with EEXIST");
});
