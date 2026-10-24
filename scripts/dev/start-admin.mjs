// Purpose: launch the packaged admin Next on the production listener name (localhost) over loopback.
// Depends on: the built apps/admin standalone server.js and Node's IPv4-first resolver flag.
// Used by: tests/admin/ui-gallery-runner.mjs, tests/admin/shell-runner.mjs and local dev; no production process.
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const port = process.env.COMMERCE_UI_PORT ?? "3100";
if (!/^\d{4,5}$/.test(port) || Number(port) > 65535 || Number(port) < 1024)
  throw new Error("Invalid UI port");
const server = fileURLToPath(
  new URL(
    "../../apps/admin/.next/standalone/apps/admin/server.js",
    import.meta.url,
  ),
);
// This launcher is local-only. Deployment ingress/listener configuration belongs
// to the deployment manifest, not an ambient machine HOSTNAME variable. Pin the
// production listener name so Next keeps locale redirects path-only (TH-3, #147),
// with IPv4-first resolution so "localhost" still binds loopback 127.0.0.1.
const child = spawn(process.execPath, ["--dns-result-order=ipv4first", server], {
  stdio: "inherit",
  env: { ...process.env, HOSTNAME: "localhost", PORT: port },
});
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => child.kill(signal));
child.on("error", () => {
  console.error("Could not start packaged admin server.");
  process.exitCode = 1;
});
child.on("exit", (code) => {
  process.exitCode = code ?? 1;
});
