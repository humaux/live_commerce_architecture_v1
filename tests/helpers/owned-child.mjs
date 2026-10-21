// Purpose: own one detached child process group for a storefront harness and clean it up exactly once, even when the harness fails
// before the child was ever spawned (port allocation / log open), without a cleanup error masking the original failure (#132).
// Depends on: Node child_process handles (caller spawns), process.kill, fs/promises rm.
// Used by: tests/storefront/ad-link.mjs; tests/ci/owned-child.test.mjs. Invariant: every method is a no-op for an unspawned child.
import { rm } from "node:fs/promises";

const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

export function createOwnedChild(label) {
  let child, closed = true, closedPromise = Promise.resolve();
  const signal = (sig) => {
    if (!child?.pid) return; // #132: unspawned child (failure before spawn) has nothing to signal
    try { if (process.platform === "win32") child.kill(sig); else process.kill(-child.pid, sig); }
    catch (err) { if (err.code !== "ESRCH") throw err; }
  };
  const groupAlive = () => {
    if (!child?.pid) return false;
    if (process.platform === "win32") return !closed;
    try { process.kill(-child.pid, 0); return true; } catch (err) { if (err.code === "ESRCH") return false; throw err; }
  };
  async function waitStopped() {
    const deadline = Date.now() + 3000;
    while ((!closed || groupAlive()) && Date.now() < deadline) await delay(50);
  }
  const emergencyStop = () => signal("SIGKILL");
  process.on("exit", emergencyStop);
  return {
    /** Register the spawned child (call right after spawn). */
    adopt(spawned) {
      child = spawned; closed = false;
      closedPromise = new Promise((resolve) => spawned.once("close", () => { closed = true; resolve(); }));
    },
    get closed() { return closed; },
    async stop() {
      signal("SIGTERM");
      await waitStopped();
      if (!closed || groupAlive()) { signal("SIGKILL"); await waitStopped(); }
      if (!closed || groupAlive()) throw new Error(`${label} cleanup failed: owned process group did not close`);
      await closedPromise;
      process.off("exit", emergencyStop);
    },
  };
}

/** Run body(owned); always stop the child and remove dir afterwards. */
export async function withOwnedChild(body, { label, dir }) {
  const owned = createOwnedChild(label);
  let failed = false;
  try { return await body(owned); }
  catch (err) { failed = true; throw err; }
  finally {
    // A cleanup failure must not replace the body's error (finally's abrupt completion wins in JS); dir removal always runs.
    try { await owned.stop(); }
    catch (err) { if (!failed) throw err; console.error(`${label}: cleanup also failed: ${err.message}`); }
    finally { await rm(dir, { recursive: true, force: true }); }
  }
}
