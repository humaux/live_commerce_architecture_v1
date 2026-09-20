import { cp, mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

// Next standalone excludes these by design; package them explicitly so the
// deployable directory does not depend on a development checkout or CDN.
const root = fileURLToPath(new URL("../../", import.meta.url));
const app = path.join(root, "apps/admin");
const output = path.join(app, ".next/standalone/apps/admin");
await mkdir(path.join(output, ".next"), { recursive: true });
await cp(path.join(app, "public"), path.join(output, "public"), {
  recursive: true,
});
await cp(path.join(app, ".next/static"), path.join(output, ".next/static"), {
  recursive: true,
});
console.log("Packaged standalone admin server and static assets.");
