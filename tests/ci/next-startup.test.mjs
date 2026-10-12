// Purpose: real loopback/socket and owned-child regression for the Next harness port handoff race.
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
  assert.deepEqual(collect(current, current, assertions), collect(old, old, assertions), `${file}: original gate assertions changed`);
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
