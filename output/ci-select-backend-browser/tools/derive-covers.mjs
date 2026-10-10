#!/usr/bin/env node
// Purpose: one-off derivation of each browser mode's lc_covers package list from what its Go harness in tests/foundation
//   actually wires: seed = the files defining the Test functions the mode's registry body runs (-run prefixes match
//   TestX* just like the real regex would); closure = file-level identifier-reference graph of the tests/foundation
//   package (function/type/var/const names; method calls link via the receiver type, never the method name, so common
//   method words cannot explode the closure); direct covers = the union of the livecommerce/internal/... imports of the
//   closure files; final covers = direct covers expanded transitively through the internal/ import graph, because a
//   harness that mounts httpapi.NewHandler constructs every service the handler transitively imports (the wiring is
//   inside the handler, not visible from tests/foundation imports alone). Closure files that build a real server/worker
//   binary (a quoted "cmd/<bin>" argument, e.g. exec go build ./cmd/api in account_process_test.go) additionally wire
//   everything that binary imports — this is how internal/tlsask, mounted only by cmd/api, is still exercised by the
//   process-backed browser modes. Over-approximation is intended (conservative selection). Diagnostics also lists, for
//   every still-uncovered internal package, the files that import it, so its BACKEND_ONLY_PACKAGES reason can be
//   honest data instead of a guess.
// Depends on: scripts/dev/pr-modes.mjs (modeEntries), tests/foundation/*.go, internal/**/*.go, git ls-files.
// Used by: unit ci-select-backend-browser (writes the lc_covers lines of scripts/dev/test-local.sh); evidence for DELIVERY.md.
import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import path from "node:path";
import { modeEntries, EXCLUDED_MODES } from "../../../scripts/dev/pr-modes.mjs";

const root = path.resolve(import.meta.dirname, "../../..");
const usage = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");
const entries = modeEntries(usage);
const BROWSER = /^--(.*browser.*|e2e)$/;

// ---- strip comments and string literals so identifier scanning sees code only ----
function strip(src) {
  let out = "", i = 0;
  const n = src.length;
  while (i < n) {
    const c = src[i];
    if (c === "/" && src[i + 1] === "/") { while (i < n && src[i] !== "\n") { out += " "; i++; } }
    else if (c === "/" && src[i + 1] === "*") { out += "  "; i += 2; while (i < n && !(src[i] === "*" && src[i + 1] === "/")) { out += src[i] === "\n" ? "\n" : " "; i++; } out += "  "; i += 2; }
    else if (c === '"') { out += " "; i++; while (i < n && src[i] !== '"') { if (src[i] === "\\") { out += " "; i++; } out += src[i] === "\n" ? "\n" : " "; i++; } out += " "; i++; }
    else if (c === "`") { out += " "; i++; while (i < n && src[i] !== "`") { out += src[i] === "\n" ? "\n" : " "; i++; } out += " "; i++; }
    else if (c === "'") { out += " "; i++; while (i < n && src[i] !== "'") { if (src[i] === "\\") { out += " "; i++; } out += " "; i++; } out += " "; i++; }
    else { out += c; i++; }
  }
  return out;
}
const internalImportRe = /"livecommerce\/(internal\/[A-Za-z0-9_./-]+)"/g;
// Quoted build/exec arguments like "./cmd/api" or "../../cmd/media-worker" (prose mentions are not quoted this way).
const cmdRefRe = /["'](?:\.\.?\/)*cmd\/([a-z0-9-]+)["']/g;

// ---- index the tests/foundation package ----
const dir = path.join(root, "tests/foundation");
const files = readdirSync(dir).filter((f) => f.endsWith(".go")).sort();
const info = new Map(); // file -> { stripped, imports:Set, defs:Set, tokens:Set }
const defOwners = new Map(); // identifier -> Set(file)
for (const f of files) {
  const raw = readFileSync(path.join(dir, f), "utf8");
  const stripped = strip(raw);
  // Import paths live inside string literals, so they must be read from raw (strip() erases them); over-matching a
  // path mentioned in a comment only widens covers, which stays conservative.
  const imports = new Set([...raw.matchAll(internalImportRe)].map((m) => m[1]));
  const cmdRefs = new Set([...raw.matchAll(cmdRefRe)].map((m) => `cmd/${m[1]}`));
  const defs = new Set();
  for (const m of stripped.matchAll(/^func (?:\(\s*\w+\s+\*?([A-Za-z_][A-Za-z0-9_]*)\s*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)/gm)) {
    if (m[1]) defs.add(m[1]);            // method: link via receiver TYPE only
    else defs.add(m[2]);                  // plain function
  }
  for (const m of stripped.matchAll(/^type\s+([A-Za-z_][A-Za-z0-9_]*)/gm)) defs.add(m[1]);
  for (const m of stripped.matchAll(/^(?:var|const)\s+([A-Za-z_][A-Za-z0-9_]*)/gm)) defs.add(m[1]);
  // var/const block entries: "\tname = ..." / "\tname Type = ..."
  for (const m of stripped.matchAll(/^(?:var|const)\s*\(\n([\s\S]*?)^\)/gm)) {
    for (const b of m[1].matchAll(/^\s*([A-Za-z_][A-Za-z0-9_]*)/gm)) defs.add(b[1]);
  }
  const tokens = new Set(stripped.match(/[A-Za-z_][A-Za-z0-9_]*/g));
  info.set(f, { raw, stripped, imports, cmdRefs, defs, tokens });
  for (const d of defs) { if (!defOwners.has(d)) defOwners.set(d, new Set()); defOwners.get(d).add(f); }
}
// adjacency: file -> files whose identifiers it references (conservative: name collisions link both)
const adj = new Map();
for (const f of files) {
  const { tokens, defs } = info.get(f);
  const dep = new Set();
  for (const t of tokens) { if (defs.has(t)) continue; for (const g of defOwners.get(t) ?? []) dep.add(g); }
  adj.set(f, dep);
}
function closure(seeds) {
  const seen = new Set(seeds);
  const queue = [...seeds];
  while (queue.length) for (const g of adj.get(queue.shift()) ?? []) if (!seen.has(g)) { seen.add(g); queue.push(g); }
  return seen;
}

// ---- internal package dirs and the internal import graph (non-test files: only they link into a test binary) ----
const tracked = execFileSync("git", ["ls-files", "-z", "--", "internal/"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 })
  .split("\0").filter(Boolean).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"));
const pkgDirs = [...new Set(tracked.map((f) => path.posix.dirname(f)))].sort();
const internalImports = new Map(); // package dir -> Set(imported package dir)
for (const f of tracked) {
  const raw = readFileSync(path.join(root, f), "utf8"); // import paths are string literals: read raw, never strip()ed
  const d = path.posix.dirname(f);
  if (!internalImports.has(d)) internalImports.set(d, new Set());
  for (const m of raw.matchAll(internalImportRe)) internalImports.get(d).add(m[1]);
}
// cmd/<bin> main packages: their internal imports (a harness that builds and runs the binary wires all of them).
const cmdFiles = execFileSync("git", ["ls-files", "-z", "--", "cmd/"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 })
  .split("\0").filter(Boolean).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"));
const cmdImports = new Map(); // "cmd/<bin>" -> Set(internal package dir)
for (const f of cmdFiles) {
  const raw = readFileSync(path.join(root, f), "utf8");
  const d = path.posix.dirname(f);
  if (!cmdImports.has(d)) cmdImports.set(d, new Set());
  for (const m of raw.matchAll(internalImportRe)) cmdImports.get(d).add(m[1]);
}

// A harness constructs everything its imported packages transitively import (httpapi.NewHandler builds services
// internally), plus everything the binaries it builds import, so expand through both graphs.
// Over-approximation stays conservative.
function expand(direct, cmdRefs) {
  const seen = new Set();
  const queue = [...direct];
  for (const c of cmdRefs) for (const p of cmdImports.get(c) ?? []) queue.push(p);
  while (queue.length) {
    const p = queue.shift();
    if (seen.has(p)) continue;
    seen.add(p);
    for (const q of internalImports.get(p) ?? []) if (!seen.has(q)) queue.push(q);
  }
  return seen;
}

// ---- per-mode derivation ----
const out = {};
const diagnostics = [];
for (const e of entries) {
  if (!BROWSER.test(e.name) || e.name in EXCLUDED_MODES) continue;
  const body = e.prepare + "\n" + e.run;
  const testNames = [...new Set([...body.matchAll(/\bTest[A-Za-z0-9_]+/g)].map((m) => m[0]))];
  // Prefix match, like the real `-run '^TestX'` regexes: TestBrowserAdsAttribution runs TestBrowserAdsAttributionAT5 too.
  const seeds = files.filter((f) => testNames.some((t) => new RegExp(`^func ${t}[A-Za-z0-9_]*\\(`, "m").test(info.get(f).stripped)));
  if (!seeds.length) { out[e.name] = []; diagnostics.push(`${e.name}: NO GO SEED (node-only harness or no Test token) -> lc_covers=""`); continue; }
  const cl = closure(seeds);
  const direct = new Set(), cmds = new Set();
  for (const f of cl) { for (const p of info.get(f).imports) direct.add(p); for (const c of info.get(f).cmdRefs) cmds.add(c); }
  const pkgs = expand(direct, cmds);
  out[e.name] = [...pkgs].sort();
  diagnostics.push(`${e.name}: seeds=${seeds.length} closure=${cl.size}/${files.length} direct=${direct.size} cmds={${[...cmds].sort().join(",")}} expanded=${pkgs.size}  [${seeds.join(", ")}]`);
}

// ---- what stays uncovered, and who imports it (data for honest BACKEND_ONLY reasons) ----
const declared = new Set(Object.values(out).flat());
const uncovered = pkgDirs.filter((d) => ![...declared].some((c) => d === c || d.startsWith(c + "/")));
const allGo = execFileSync("git", ["ls-files", "-z", "--", "*.go"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 }).split("\0").filter(Boolean);
const importers = new Map(); // uncovered pkg -> Set(importing file)
for (const f of allGo) {
  const raw = readFileSync(path.join(root, f), "utf8");
  for (const m of raw.matchAll(internalImportRe)) {
    // Longest match: an import of internal/x/y belongs to "internal/x/y", not to its parent "internal/x".
    const hits = uncovered.filter((u) => m[1] === u || m[1].startsWith(u + "/")).sort((a, b) => b.length - a.length);
    const hit = hits[0];
    if (hit) { if (!importers.has(hit)) importers.set(hit, new Set()); importers.get(hit).add(f); }
  }
}

writeFileSync(path.join(import.meta.dirname, "covers.json"), JSON.stringify(out, null, 1));
const report = [
  ...diagnostics,
  "",
  `internal package dirs: ${pkgDirs.length}; covered by some browser mode (transitively expanded): ${pkgDirs.length - uncovered.length}; uncovered: ${uncovered.length}`,
  ...uncovered.map((u) => `  ${u} <- imported by: ${[...(importers.get(u) ?? [])].sort().join(", ") || "(no Go importer found)"}`),
  "",
].join("\n");
writeFileSync(path.join(import.meta.dirname, "diagnostics.txt"), report);
console.log(report);
