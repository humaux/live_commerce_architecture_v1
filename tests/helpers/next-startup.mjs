// Purpose: shared owned-Next startup boundary for browser gates; retry only automatic-port bind collisions.
// Depends on: Node net/events/fetch; caller-owned child/log and unchanged caller readiness callback.
// Used by: tests/storefront/*-gate.mjs; tests/ci/next-startup.test.mjs. No production process is managed here.
import net from "node:net";
import { once } from "node:events";
import { open } from "node:fs/promises";

/** Pick an ephemeral loopback port. The reservation cannot survive exec: callers must still handle bind races. */
export async function allocateNextPort() {
  const probe = net.createServer();
  probe.listen(0, "127.0.0.1");
  await once(probe, "listening");
  const port = probe.address().port;
  await new Promise((resolve, reject) => probe.close((error) => error ? reject(error) : resolve()));
  return port;
}

/** Keep the first log filename stable, while every later attempt preserves its own exclusive log. */
export function nextAttemptLog(file, attempt) {
  return attempt === 1 ? file : `${file}.attempt-${attempt}`;
}

/** Attach a ready automatic-port admin to its Go-owned public origin: ONE bounded 5 s attempt (K3 P2: 1 s was tight on a loaded CI host); never retry/restart on a lost acknowledgement. */
export async function connectFixtureAdmin(port, control, key) {
  if (!Number.isInteger(port) || port < 1 || port > 65535 || !/^http:\/\/127\.0\.0\.1:\d+$/.test(control) || !key)
    throw new Error("invalid fixture admin registration");
  const response = await fetch(`${control}/admin-upstream`, {
    method: "POST", headers: { "X-Gate-Key": key, "Content-Type": "application/json" },
    body: JSON.stringify({ port }), signal: AbortSignal.timeout(5000),
  });
  if (response.status !== 204) throw new Error(`fixture admin registration refused (${response.status})`);
}

async function bindCollision(log) {
  if (!log || typeof log.path !== "string") return false;
  let file;
  try {
    if (!log.closed && !log.destroyed) { const closed = once(log, "close"); log.end(); await closed; }
    file = await open(log.path, "r");
    const size = (await file.stat()).size;
    const buffer = Buffer.alloc(Math.min(size, 64 * 1024)); // bounded startup diagnostics, never an unbounded log read
    const { bytesRead } = await file.read(buffer, 0, buffer.length, Math.max(0, size - buffer.length));
    return /\bEADDRINUSE\b/.test(buffer.subarray(0, bytesRead).toString("utf8"));
  } catch { return false; } // no proof => original failure, never a speculative restart
  finally { await file?.close().catch(() => {}); }
}

/** Retry at most three launches, only automatic-port EADDRINUSE exits before the caller's unchanged readiness succeeds. */
export async function startNextWithPortRetry({ port, allocatePort = allocateNextPort } = {}, start) {
  const explicit = Boolean(port); // existing gates pass Number(optional env), so 0/NaN still mean automatic
  for (let attempt = 1; attempt <= 3; attempt++) {
    const selected = explicit ? port : await allocatePort();
    if (!Number.isInteger(selected) || selected < 1 || selected > 65535) throw new RangeError("invalid Next port");
    let child, log;
    try {
      return await start({ port: selected, attempt, track(ownedChild, ownedLog) {
        child = ownedChild; log = ownedLog;
      } });
    } catch (error) {
      const exited = child && (child.exitCode !== null || child.signalCode !== null);
      if (explicit || attempt === 3 || !exited || !(await bindCollision(log))) throw error;
    }
  }
}
