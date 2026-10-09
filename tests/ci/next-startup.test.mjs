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
import { allocateNextPort, nextAttemptLog, startNextWithPortRetry } from "../helpers/next-startup.mjs";

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

test("all eleven gate adapters preserve original assertions, readiness loops, spawn and environment statements", async () => {
  const baseline = "9b738e0af64ebd08c25b39f8081d46d4dc5674fb";
  const files = execFileSync("git", ["ls-tree", "-r", "--name-only", baseline, "tests/storefront"], { encoding: "utf8" }).trim().split("\n").filter((f) => f.endsWith("-gate.mjs"));
  const printer = ts.createPrinter({ removeComments: true });
  let checked = 0;
  for (const file of files) {
    const before = execFileSync("git", ["show", `${baseline}:${file}`], { encoding: "utf8" });
    const old = ts.createSourceFile(file, before, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
    const start = old.statements.find((n) => ts.isFunctionDeclaration(n) && ["startNext", "startStorefront"].includes(n.name?.text));
    if (!start) continue;
    checked++;
    const current = ts.createSourceFile(file, await readFile(file, "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
    const text = (node, source) => printer.printNode(ts.EmitHint.Unspecified, node, source);
    const collect = (source, root, predicate) => {
      const out = []; const visit = (node) => { if (predicate(node, source)) out.push(text(node, source)); ts.forEachChild(node, visit); }; visit(root); return out;
    };
    const now = current.statements.find((n) => ts.isFunctionDeclaration(n) && n.name?.text === start.name.text);
    const assertions = (n, source) => ts.isCallExpression(n) && /^(assert|expect)(?:\.|\(|$)/.test(n.expression.getText(source));
    assert.deepEqual(collect(current, current, assertions), collect(old, old, assertions), `${file}: original gate assertions changed`);
    assert.deepEqual(collect(current, now, ts.isForStatement), collect(old, start, ts.isForStatement), `${file}: readiness/env loop changed`);
    const spawnCall = (n, source) => ts.isCallExpression(n) && n.expression.getText(source) === "spawn";
    assert.deepEqual(collect(current, now, spawnCall), collect(old, start, spawnCall), `${file}: spawned app/args/env changed`);
    const envStatement = (n, source) => ts.isVariableStatement(n) && n.declarationList.declarations.some((d) => ["env", "childEnv", "args", "bin"].includes(d.name.getText(source)));
    assert.deepEqual(collect(current, now, envStatement), collect(old, start, envStatement), `${file}: environment/command changed`);
    assert.ok(collect(current, now, (n, source) => ts.isCallExpression(n) && n.expression.getText(source) === "startNextWithPortRetry").length === 1, `${file}: shared helper missing`);
    assert.ok(!/net\.createServer\(|freePort\(|listen\((reserve|probe)\)/.test(now.getText(current)), `${file}: old reservation survives`);
  }
  assert.equal(checked, 11);
});
