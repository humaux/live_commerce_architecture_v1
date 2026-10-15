// Purpose: design-spec rules R1–R7 (docs/design/DESIGN-SPEC.md §6) enforced while developing: DS component files and every page marked `@ds-page`
//   may not hard-code colours, spacing or font sizes, draw icons with text glyphs or raw <svg>, use inline style objects or plain className strings.
// Depends on: node:fs/path, stripComments from ./check-hex-ratchet.mjs (comment-safe scanning); reads sources only.
// Used by: scripts/dev/test-node.sh (CLI, exits 1 on any violation), tests/ui/ds-rules.test.mjs (pure functions + injected violations), integrator prepush.
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { extname, join } from "node:path";
import { stripComments } from "./check-hex-ratchet.mjs";

export const DS_DIR = "packages/ui/src/ds";
export const PAGE_ROOT = "apps/admin";
export const MARK = "@ds-page";
const ICON_FILE = /(^|\/)icons\.tsx$/;
// §2: text characters that were used as icons in DS-1 (and their usual look-alikes). Arrows such as ↑ stay legal inside text ("↑ 130%").
export const GLYPHS = "⌂≡✉▣☺✦⚙•▸▾◂▴▲▼◀▶⌕✓✔✕✖×○●■□⚿⦿⛯❯❮›‹⋯⋮";
const GLYPH_CLASS = `[${GLYPHS}]`;
const SPACE_PROPS = /^(?:margin|padding|gap|row-gap|column-gap)(?:-(?:top|right|bottom|left|inline|block|inline-start|inline-end|block-start|block-end))?$/;
const SPACE_OK = /^(?:var\(--ds-space-[\w-]+\)|0|0px|1px|auto|-?\d+(?:\.\d+)?%|calc\(.*var\(--ds-space-.*\))$/;

/** Violations in one file's source: [{rule, line, text}]. `ext` picks the CSS or TSX rule set. */
export function check(source, ext, file = "") {
  const out = [];
  const clean = stripComments(source, ext);
  const lines = clean.split("\n");
  const add = (rule, i, text) => out.push({ rule, line: i + 1, text: text.trim().slice(0, 120) });
  if (ext === ".css" || ext === ".scss") {
    lines.forEach((l, i) => {
      if (/(?<![\w&#-])#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})(?![\w-])|\b(?:rgba?|hsla?)\(/.test(l)) add("R1 colour literal", i, l);
      for (const m of l.matchAll(/([a-z-]+)\s*:\s*([^;{}]+)/g)) {
        const [prop, raw] = [m[1], m[2].replace(/!important/, "").trim()];
        if (SPACE_PROPS.test(prop)) {
          // calc(...) may nest var(): accept a whole calc value that only combines --ds-space tokens with numbers; otherwise check each part.
          const ok = /^calc\(/.test(raw) ? /var\(--ds-space-/.test(raw) && !/\d+(?:px|rem|em)(?![\w-])/.test(raw.replace(/\b1px\b/g, ""))
            : (raw.match(/var\([^)]*\)|\S+/g) || []).every((p) => SPACE_OK.test(p));
          if (!ok) add("R2 spacing not a --ds-space token", i, l);
        }
        if (prop === "font-size" && !/^(?:var\(--ds-text-[\w-]+\)|inherit|100%|1em)$/.test(raw)) add("R3 font-size not a --ds-text token", i, l);
      }
    });
    return out;
  }
  // TSX / TS
  lines.forEach((l, i) => {
    if (/["'`]#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})["'`]|["'`](?:rgba?|hsla?)\(/.test(l)) add("R1 colour literal", i, l);
    if (!ICON_FILE.test(file)) {
      if (new RegExp(`>\\s*${GLYPH_CLASS}\\s*<|["'\`]\\s*${GLYPH_CLASS}\\s*["'\`]|>\\s*${GLYPH_CLASS}\\s+\\S`).test(l)) add("R4 text glyph used as an icon", i, l);
      if (/<svg[\s>]/.test(l)) add("R5 raw <svg> outside icons.tsx", i, l);
    }
    for (const m of l.matchAll(/style=\{\{([^}]*)\}\}/g)) {
      const keys = [...m[1].matchAll(/(?:^|,)\s*(["']?)([\w-]+)\1\s*:/g)].map((k) => k[2]);
      if (keys.some((k) => !k.startsWith("--"))) add("R6 inline style (only --custom properties allowed)", i, l);
    }
    if (/className=["'][^"']+["']/.test(l)) add("R7 plain className string (use CSS modules or DS components)", i, l);
  });
  return out;
}

function walk(dir, acc = []) {
  if (!existsSync(dir)) return acc;
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    if (e.name === "node_modules" || e.name === ".next") continue;
    const p = join(dir, e.name);
    if (e.isDirectory()) walk(p, acc);
    else if (/\.(css|scss|tsx|ts)$/.test(e.name)) acc.push(p);
  }
  return acc;
}

/** Files in scope: everything under packages/ui/src/ds, plus apps/admin sources whose text carries the @ds-page marker. */
export function scope() {
  const ds = walk(DS_DIR);
  const pages = walk(PAGE_ROOT).filter((f) => !f.includes(`${PAGE_ROOT}/tests/`) && readFileSync(f, "utf8").includes(MARK));
  return [...ds, ...pages];
}

export function scan(files = scope()) {
  return files.flatMap((f) => check(readFileSync(f, "utf8"), extname(f), f).map((v) => ({ file: f, ...v })));
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const files = scope();
  const v = scan(files);
  for (const x of v) console.error(`${x.file}:${x.line} ${x.rule}: ${x.text}`);
  console.log(`check-ds-rules: ${files.length} files in scope, ${v.length} violation(s)`);
  process.exit(v.length ? 1 : 0);
}
