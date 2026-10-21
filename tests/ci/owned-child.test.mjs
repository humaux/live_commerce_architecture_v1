// Purpose: #132 regression - owned-child cleanup must survive a failure before the first spawn: the ORIGINAL error propagates (no
// TypeError from an unspawned child), the temp dir is removed, no exit listener leaks; a spawned child is still reaped.
// Depends on: tests/helpers/owned-child.mjs; Node child_process/fs/os/test. Synthetic `node -e` sleeper only, no network.
// Used by: scripts/dev/test-node.sh.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { EventEmitter } from "node:events";
import { existsSync } from "node:fs";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { withOwnedChild } from "../helpers/owned-child.mjs";

const tmp = () => mkdtemp(path.join(tmpdir(), "lc-owned-child-"));

test("#132: a failure before the first spawn keeps the original error and still cleans up", async () => {
  const dir = await tmp(), exitListeners = process.listenerCount("exit");
  const original = new Error("port allocation failed");
  await assert.rejects(withOwnedChild(async () => { throw original; }, { label: "T", dir }), (err) => err === original);
  assert.equal(existsSync(dir), false, "temp dir must be removed");
  assert.equal(process.listenerCount("exit"), exitListeners, "exit listener must be unregistered");
});

test("#132: a spawned child is reaped when the body later fails, original error preserved", async () => {
  const dir = await tmp(), original = new Error("readiness failed");
  let pid;
  await assert.rejects(withOwnedChild(async (owned) => {
    const child = spawn(process.execPath, ["-e", "setInterval(()=>{},1000)"], { stdio: "ignore", detached: process.platform !== "win32" });
    owned.adopt(child); pid = child.pid;
    throw original;
  }, { label: "T", dir }), (err) => err === original);
  assert.throws(() => process.kill(pid, 0), { code: "ESRCH" }, "child must be dead");
  assert.equal(existsSync(dir), false);
});

test("#132: when the body AND stop() both fail, the original body error propagates and the dir is still removed", async () => {
  const dir = await tmp(), original = new Error("body failed");
  // A fake child that never emits "close" (pid beyond pid_max, so no real process is signalled) makes stop() throw after its
  // normal 2x3s grace; kill timings are untouched.
  const never = Object.assign(new EventEmitter(), { pid: 4194999 });
  const log = console.error; let logged = "";
  console.error = (msg) => { logged += msg; };
  try {
    await assert.rejects(withOwnedChild(async (owned) => { owned.adopt(never); throw original; }, { label: "T", dir }), (err) => err === original);
  } finally { console.error = log; }
  assert.match(logged, /T: cleanup also failed: T cleanup failed: owned process group did not close/);
  assert.equal(existsSync(dir), false);
});
