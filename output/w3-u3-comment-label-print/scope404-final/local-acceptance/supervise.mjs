// Purpose: keep this approved W3-U3 serial acceptance alive across chat/heartbeat interruptions.
// Depends on: Node stdlib, the existing test-local registry/heartbeat lock and frozen source4f0e08e5.
// Used by: codex-w3-u3 and its existing heartbeat; no test operation, assertion or timeout is changed.
import { spawn, execFileSync } from "node:child_process";
import { mkdirSync, openSync, closeSync, writeFileSync, renameSync } from "node:fs";
import path from "node:path";
const root = "/Volumes/data/live_commerce_architecture_v1/.worktrees/w3-u3-comment-label-print";
const expected = "4f0e08e5003838d2746ce7dc9c77d1cccf7471e3";
const base = path.join(root, "output/w3-u3-comment-label-print/scope404-final/local-acceptance");
const self = path.join(base, "supervise.mjs");
if (process.argv[2] === "--launch") {
  const dir = path.join(base, "restart-" + new Date().toISOString().replace(/[-:.]/g, ""));
  mkdirSync(dir);
  const fd = openSync(path.join(dir, "supervisor.log"), "wx", 0o600);
  const child = spawn(process.execPath, [self, dir], { cwd: root, detached: true, stdio: ["ignore", fd, fd] });
  closeSync(fd); child.unref();
  console.log(JSON.stringify({ pid: child.pid, dir, expected }));
} else {
  const dir = process.argv[2];
  if (!dir?.startsWith(base + "/restart-")) throw new Error("invalid own run directory");
  const state = { source: expected, pid: process.pid, started_at: new Date().toISOString(), updated_at: "", phase: "initializing", runs: [], completed: false };
  const save = () => { state.updated_at = new Date().toISOString(); const tmp = path.join(dir, "state.tmp"); writeFileSync(tmp, JSON.stringify(state, null, 2)); renameSync(tmp, path.join(dir, "state.json")); };
  const heartbeat = setInterval(save, 15000);
  let failures = 0;
  try {
    for (const [tag, mode, goBudget] of [
      ["console-1", "--browser-live-console", 780], ["console-2", "--browser-live-console", 780],
      ["inbox", "--browser-inbox", 1200], ["click-sweep", "--browser-click-sweep", 5400], ["visual-lint", "--browser-visual-lint", 5400],
    ]) {
      if (execFileSync("git", ["rev-parse", "HEAD"], { cwd: root, encoding: "utf8" }).trim() !== expected) throw new Error("source HEAD changed");
      execFileSync("git", ["diff", "--quiet", expected, "--", "apps", "tests", "scripts/dev"], { cwd: root });
      const env = { ...process.env, LC_TEST_LOCK_WAIT: "14400" };
      for (const key of ["LC_BROWSER_CONSOLE_GREP", "LC_CONSOLE_CALIBRATION", "LC_SWEEP_SHARD"]) delete env[key];
      const log = path.join(dir, tag + ".log");
      const fd = openSync(log, "wx", 0o600);
      const run = { tag, mode, source: expected, command: "LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh " + mode, started_at: new Date().toISOString(), log, go_budget_seconds: goBudget, supervisor_timeout_seconds: 14400 + goBudget + 1800, exit: null };
      state.runs.push(run); state.phase = tag; save();
      const child = spawn("bash", ["scripts/dev/test-local.sh", mode], { cwd: root, env, detached: true, stdio: ["ignore", fd, fd] });
      closeSync(fd); run.pid = child.pid; save();
      let timedOut = false;
      const timeout = setTimeout(() => { timedOut = true; if (child.exitCode === null) { try { process.kill(-child.pid, "SIGTERM"); } catch {} } }, run.supervisor_timeout_seconds * 1000);
      const result = await new Promise((resolve) => {
        child.once("error", (error) => resolve({ code: 2, error: error.code || "spawn_error" }));
        child.once("close", (code, signal) => resolve({ code: timedOut ? 124 : (code ?? 2), signal, timed_out: timedOut }));
      });
      clearTimeout(timeout); Object.assign(run, result, { exit: result.code, ended_at: new Date().toISOString() });
      if (result.code !== 0) failures++;
      save(); console.log(JSON.stringify(run));
    }
    state.completed = true; state.phase = "finished"; state.exit = failures ? 1 : 0;
  } catch (error) {
    state.phase = "supervisor_error"; state.error = error.message; state.exit = 2; state.completed = true;
  } finally { clearInterval(heartbeat); save(); console.log(JSON.stringify({ completed: state.completed, exit: state.exit, failures })); }
  process.exitCode = state.exit;
}
