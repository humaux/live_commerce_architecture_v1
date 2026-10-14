// Purpose: DS-1 ratchet: the number of hard-coded hex colours in apps/admin CSS/TS/TSX may only go down against the committed baseline tests/ui/hex-baseline.json (total and per file); new files start at zero. packages/ui/src/tokens.css is exempt (it defines the tokens) and lives outside the scanned tree.
// Depends on: node:fs/path only; reads apps/admin sources, never writes them.
// Used by: scripts/dev/test-node.sh (check mode), tests/ui/ds-hex-ratchet.test.mjs (pure functions); `node scripts/dev/check-hex-ratchet.mjs --write` rewrites the baseline, and only when nothing grew.
import { existsSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export const ROOT = "apps/admin";
export const BASELINE = "tests/ui/hex-baseline.json";
const SKIP_DIRS = new Set(["node_modules", ".next", "tests", "public"]);
const EXT = /\.(css|tsx|ts|mjs)$/;
// #rgb #rgba #rrggbb #rrggbbaa, not part of a word, an HTML entity or an id such as #main-content.
const HEX = /(?<![\w&#-])#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{4}|[0-9a-fA-F]{3})(?![\w-])/g;

/** Hex colours in one source text, ignoring block comments and whole-line // comments. */
export function countHex(source) {
  const code = source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
  return (code.match(HEX) ?? []).length;
}

function walk(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? (SKIP_DIRS.has(e.name) ? [] : walk(join(dir, e.name))) : EXT.test(e.name) && !e.name.endsWith(".d.ts") ? [join(dir, e.name)] : [],
  );
}

/** { total, files: { path: count } } for every file with at least one hex colour, paths sorted. */
export function scan(root = ROOT) {
  const files = {};
  for (const path of walk(root).sort()) {
    const n = countHex(readFileSync(path, "utf8"));
    if (n > 0) files[path] = n;
  }
  return { total: Object.values(files).reduce((a, b) => a + b, 0), files };
}

/** Errors when the total or any single file grew against the baseline; a file missing from the baseline has an allowance of 0. */
export function compare(current, baseline) {
  const errors = [];
  if (current.total > baseline.total) errors.push(`total hex colours ${current.total} > baseline ${baseline.total}`);
  for (const [path, n] of Object.entries(current.files)) {
    const allowed = baseline.files[path] ?? 0;
    if (n > allowed) errors.push(`${path}: ${n} hex colours > baseline ${allowed} (use a --ds-* token from packages/ui/src/tokens.css)`);
  }
  return errors;
}

if (process.argv[1]?.endsWith("check-hex-ratchet.mjs")) {
  const current = scan();
  if (process.argv.includes("--write")) {
    const previous = existsSync(BASELINE) ? JSON.parse(readFileSync(BASELINE, "utf8")) : undefined;
    const errors = previous ? compare(current, previous) : [];
    if (errors.length) {
      console.error(`check-hex-ratchet: refusing to raise the baseline:\n${errors.join("\n")}`);
      process.exit(1);
    }
    writeFileSync(BASELINE, `${JSON.stringify(current, null, 2)}\n`);
    console.log(`check-hex-ratchet: baseline written, ${current.total} hex colours in ${Object.keys(current.files).length} files`);
  } else {
    const errors = compare(current, JSON.parse(readFileSync(BASELINE, "utf8")));
    if (errors.length) {
      console.error(`check-hex-ratchet: FAIL\n${errors.join("\n")}`);
      process.exit(1);
    }
    console.log(`check-hex-ratchet: ok (${current.total} hex colours, baseline never raised; lower it with --write after removing some)`);
  }
}
