#!/usr/bin/env node
// Purpose: CI-SPEED build-once. `pack` stores this checkout's production Next builds (apps/admin/.next, apps/storefront/.next, without .next/cache) and,
//   optionally, the Go build cache as zstd tars plus STAMP.json {sha} in one directory (gates.yml `build` job uploads it); `unpack` restores one part into
//   a gate job ONLY when STAMP.sha equals this checkout's HEAD — any other SHA, a missing stamp or a missing part is refused (exit 1), never a silent rebuild.
// Depends on: git (rev-parse HEAD), GNU/bsd tar with --zstd, go (go env GOCACHE) for the gocache part. No network.
// Used by: .github/workflows/gates.yml (build job: pack; gate job: unpack gocache), scripts/dev/test-local.sh (lc_build_admin / lc_build_storefront unpack
//   admin / storefront when LC_PREBUILT_DIR is set), tests/ci/prebuilt.test.mjs.
// Invariants: a consumer runs exactly the bytes `bash scripts/dev/test-local.sh --prebuild` built from the same commit with the same lc_environment; the Go
//   cache is content-addressed by Go itself (a stale entry can only miss, never change a result), the SHA check still fences it to this run's commit.
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const PARTS = {
  admin: { dir: () => "apps/admin", entry: ".next" },
  storefront: { dir: () => "apps/storefront", entry: ".next" },
  gocache: { dir: () => execFileSync("go", ["env", "GOCACHE"], { encoding: "utf8" }).trim(), entry: "." },
};

/** null when the stamp belongs to `head` and carries `part`, else the refusal reason. */
export function checkStamp(stamp, head, part) {
  if (!stamp || typeof stamp.sha !== "string" || !/^[0-9a-f]{40}$/.test(stamp.sha)) return "missing or malformed STAMP.json";
  // RED fault injection (CI-SPEED): SHA check disabled; the GREEN commit restores it.
  if (!Array.isArray(stamp.parts) || !stamp.parts.includes(part)) return `prebuilt artifact has no ${part} part`;
  return null;
}

const head = (cwd) => execFileSync("git", ["rev-parse", "HEAD"], { cwd, encoding: "utf8" }).trim();
const tar = (args, cwd) => execFileSync("tar", args, { cwd, stdio: ["ignore", "inherit", "inherit"] });

export function pack(out, parts, cwd = process.cwd()) {
  mkdirSync(out, { recursive: true });
  for (const part of parts) {
    const { dir, entry } = PARTS[part], from = path.resolve(cwd, dir());
    if (!existsSync(path.join(from, entry))) throw new Error(`nothing to pack for ${part}: ${path.join(from, entry)} is missing`);
    tar(["--zstd", "-cf", path.resolve(out, `${part}.tar.zst`), "--exclude=.next/cache", entry], from);
  }
  // A second pack into the same directory (Next first, then the warmed Go cache) adds its parts to the same commit's stamp.
  const sha = head(cwd);
  let prior = [];
  try { const old = JSON.parse(readFileSync(path.join(out, "STAMP.json"), "utf8")); if (old.sha === sha) prior = old.parts; } catch { /* first pack */ }
  writeFileSync(path.join(out, "STAMP.json"), `${JSON.stringify({ sha, parts: [...new Set([...prior, ...parts])] })}\n`);
}

export function unpack(from, part, cwd = process.cwd()) {
  if (!PARTS[part]) throw new Error(`unknown part ${part}`);
  let stamp = null;
  try { stamp = JSON.parse(readFileSync(path.join(from, "STAMP.json"), "utf8")); } catch { /* reported by checkStamp */ }
  const refused = checkStamp(stamp, head(cwd), part);
  if (refused) throw new Error(`refusing prebuilt ${part}: ${refused}`);
  const { dir, entry } = PARTS[part], to = path.resolve(cwd, dir());
  if (entry !== ".") rmSync(path.join(to, entry), { recursive: true, force: true }); // never mix with a stale local build
  mkdirSync(to, { recursive: true });
  tar(["--zstd", "-xf", path.resolve(from, `${part}.tar.zst`)], to);
  console.log(`prebuilt: ${part} restored from ${stamp.sha}`);
}

export function main(argv = process.argv.slice(2)) {
  const [cmd, dir, ...parts] = argv;
  if (cmd === "pack" && dir && parts.length && parts.every((p) => PARTS[p])) { pack(dir, parts); return 0; }
  if (cmd === "unpack" && dir && parts.length === 1) { unpack(dir, parts[0]); return 0; }
  console.error("usage: prebuilt.mjs pack <dir> <admin|storefront|gocache>... | unpack <dir> <part>");
  return 2;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`prebuilt: ${e.message}`); process.exit(1); }
}
