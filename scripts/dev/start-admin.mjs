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
// to the deployment manifest, not an ambient machine HOSTNAME variable.
const child = spawn(process.execPath, [server], {
  stdio: "inherit",
  env: { ...process.env, HOSTNAME: "127.0.0.1", PORT: port },
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
