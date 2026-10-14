// Purpose: DS-1 ratchet: the number of hard-coded hex colours in apps/admin CSS/SCSS/JS/JSX/TS/TSX/MJS may only go down against the committed baseline tests/ui/hex-baseline.json (total and per file); new files start at zero. packages/ui/src/tokens.css is exempt (it defines the tokens) and lives outside the scanned tree.
// Depends on: node:fs/path only; reads apps/admin sources, never writes them.
// Used by: scripts/dev/test-node.sh (check mode), tests/ui/ds-hex-ratchet.test.mjs (pure functions); `node scripts/dev/check-hex-ratchet.mjs --write` rewrites the baseline, and only when nothing grew.
import { existsSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { extname, join } from "node:path";

export const ROOT = "apps/admin";
export const BASELINE = "tests/ui/hex-baseline.json";
// Skipped anywhere: dependency and build output. Skipped by exact path under the scan root only: the real test and public asset dirs
// (a folder merely named tests/public deeper in the tree, e.g. src/tests or components/public, is source and is scanned).
const SKIP_ANYWHERE = new Set(["node_modules", ".next"]);
const SKIP_UNDER_ROOT = ["tests", "public"];
const EXT = /\.(css|scss|tsx|ts|jsx|js|mjs)$/;
// #rgb #rgba #rrggbb #rrggbbaa, not part of a word, an HTML entity or an id such as #main-content.
const HEX = /(?<![\w&#-])#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{4}|[0-9a-fA-F]{3})(?![\w-])/g;

/**
 * Is the `//` at `i` certainly a line comment? Conservative: when unsure it is NOT one, because dropping the rest of a line hides
 * real colours (a false "comment" lets a new hard-coded colour pass), while counting a colour in a real comment only fails the gate loudly.
 * Never a comment: right after `:` (http://, https://) in any source, or right after `(` in SCSS (url(//cdn/x.png)); in JSX/TSX also
 * not when the line so far has a `<` (possible JSX text) and does not end in `;` `,` `{` `}` `(` `)`.
 */
function isLineComment(source, i, out, ext) {
  const prev = source[i - 1];
  if (prev === ":") return false;
  if (ext === ".scss" && prev === "(") return false;
  if (ext === ".tsx" || ext === ".jsx") {
    const lineSoFar = out.slice(out.lastIndexOf("\n") + 1).trimEnd();
    if (lineSoFar.includes("<") && !/[;,{}()]$/.test(lineSoFar)) return false;
  }
  return true;
}

/**
 * Source with comments blanked, found by a tokenizer rather than a regex: a quote starts a string (a `/*` or `//` inside it is text),
 * a `//` comment swallows a `/*` on its line, template literals nest `${ ... }` code. CSS has no `//` comments; SCSS has them but
 * `url(http://x)` is not one. Newlines are kept. Regex literals are not tokenised (none holds a quote-unbalanced hex in this tree).
 */
export function stripComments(source, ext = ".ts") {
  const line = ext !== ".css";
  const template = ext !== ".css" && ext !== ".scss";
  const modes = []; // "tpl" inside a template literal, {depth} inside a ${ } expression
  let out = "";
  let i = 0;
  while (i < source.length) {
    const ch = source[i];
    const next = source[i + 1];
    const top = modes.at(-1);
    if (top === "tpl") {
      if (ch === "\\") {
        out += source.slice(i, i + 2);
        i += 2;
      } else if (ch === "`") {
        modes.pop();
        out += ch;
        i++;
      } else if (ch === "$" && next === "{") {
        modes.push({ depth: 0 });
        out += "${";
        i += 2;
      } else {
        out += ch;
        i++;
      }
      continue;
    }
    if (ch === '"' || ch === "'") {
      let j = i + 1;
      while (j < source.length && source[j] !== ch && source[j] !== "\n") j += source[j] === "\\" ? 2 : 1;
      out += source.slice(i, j + 1);
      i = j + 1;
    } else if (ch === "`" && template) {
      modes.push("tpl");
      out += ch;
      i++;
    } else if (ch === "/" && next === "*") {
      const end = source.indexOf("*/", i + 2);
      const stop = end < 0 ? source.length : end + 2;
      out += source.slice(i, stop).replace(/[^\n]/g, " ");
      i = stop;
    } else if (ch === "/" && next === "/" && line && isLineComment(source, i, out, ext)) {
      while (i < source.length && source[i] !== "\n") i++;
    } else if (ch === "/" && next === "/" && line) {
      // uncertain `//` (URL, JSX text): keep it as code and go on, so the rest of the line is still counted
      out += "//";
      i += 2;
    } else {
      if (top && ch === "{") top.depth++;
      else if (top && ch === "}") {
        if (top.depth === 0) modes.pop();
        else top.depth--;
      }
      out += ch;
      i++;
    }
  }
  return out;
}

/** Hex colours in one source text, outside comments (see stripComments). */
export function countHex(source, ext = ".ts") {
  return (stripComments(source, ext).match(HEX) ?? []).length;
}

function walk(dir, root) {
  const skipped = SKIP_UNDER_ROOT.map((name) => join(root, name));
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const path = join(dir, e.name);
    if (e.isDirectory()) return SKIP_ANYWHERE.has(e.name) || skipped.includes(path) ? [] : walk(path, root);
    return EXT.test(e.name) && !e.name.endsWith(".d.ts") ? [path] : [];
  });
}

/** { total, files: { path: count } } for every file with at least one hex colour, paths sorted. */
export function scan(root = ROOT) {
  const files = {};
  for (const path of walk(root, root).sort()) {
    const n = countHex(readFileSync(path, "utf8"), extname(path));
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
