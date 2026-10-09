#!/usr/bin/env node
// Purpose: round-2/3 (owner decision 2026-10-10 "Narrow per domain + nightly") evidence derivation of each browser mode's
//   lc_covers DOMAIN packages, replacing round 1's import-closure covers (48/51 modes covered all 70 internal packages
//   because every harness boots the full httpapi.NewHandler, which made any backend PR as heavy as a UI PR).
//   Evidence per mode, never guesses:
//   1. API paths its Playwright specs / node runners and its Go harness actually call: URL literals in the mode's spec
//      files (registry-referenced, referenced by its Go harness files, or selected via LC_BROWSER_SUITE in
//      playwright.config.ts) and in the harness's own Go files. Admin BFF URLs (/api/stores/<store>/REST) map to Go
//      /v1/admin/stores/<store>/REST (apps/admin/lib/backend.ts callBackend pass-through); storefront /api/buyer/REST
//      maps to /v1/buyer/REST (apps/storefront/lib/buyer-server.ts); other BFF URLs map through the /v1/ literals of
//      their apps/*/app/**/route.ts handler (+ its one-level lib imports). Each Go path is matched against the real
//      route registrations: internal/httpapi + internal/identityhttp mux.HandleFunc patterns (const base resolution,
//      helper/receiver/param type attribution) and internal/buyerhttp's matchRoute kind dispatch (kind -> serve block
//      -> service packages). Handler file:line recorded for every path.
//   2. Go fixture calls: the direct livecommerce/internal imports of the mode's OWN harness files (the files defining
//      its -run Test functions, files the registry names, and files whose header says "Used by: <its tests/mode>"),
//      plus the direct internal imports of the cmd/ binaries those files build/run (quoted "./cmd/<bin>" arguments).
//      Deliberately NOT the transitive internal closure (that was round 1's explosion) and NOT every file of the
//      tests/foundation identifier graph (shared boot helpers would re-widen every mode): service-level coupling below
//      the called handlers is the nightly full-matrix's job (owner's safety net).
//   Outputs: covers-derivation.json (mode -> [{path, handler, package, via}], the task deliverable), covers-narrow.json
//   (mode -> final lc_covers list = evidence union minus SHARED_BACKEND_PACKAGES, which select every PG mode anyway),
//   r2-diagnostics.txt (unmapped URLs, per-package mode counts, uncovered packages with their importers — the data for
//   SHARED/BACKEND_ONLY decisions). SHARED_BACKEND_PACKAGES is imported from scripts/dev/pr-modes.mjs, so the tool is
//   re-run after that list is finalised and stays reproducible.
// Depends on: scripts/dev/pr-modes.mjs (modeEntries, browserModes, SHARED_BACKEND_PACKAGES), scripts/dev/test-local.sh,
//   playwright.config.ts, internal/{httpapi,identityhttp,buyerhttp}/*.go, tests/foundation/*.go, cmd/**/*.go,
//   apps/{admin,storefront} route/lib sources, git ls-files.
// Used by: unit ci-select-backend-browser round 2 (writes the lc_covers lines of scripts/dev/test-local.sh via
//   insert-narrow-covers.mjs); evidence for DELIVERY.md. Run: node output/ci-select-backend-browser/tools/derive-narrow-covers.mjs
import { readFileSync, readdirSync, writeFileSync, existsSync } from "node:fs";
import { execFileSync } from "node:child_process";
import path from "node:path";
import { modeEntries, browserModes, SHARED_BACKEND_PACKAGES, BACKEND_ONLY_PACKAGES, FILE_CLASSIFIED_PACKAGES, backendPathMatches } from "../../../scripts/dev/pr-modes.mjs";

const root = path.resolve(import.meta.dirname, "../../..");
const read = (rel) => readFileSync(path.join(root, rel), "utf8");

// ---------- masked views (same length as the source, so offsets/lines stay valid) ----------
function mask(src, { comments = true, strings = true } = {}) {
  let out = "", i = 0;
  const n = src.length;
  const blank = (ch) => (ch === "\n" ? "\n" : " ");
  while (i < n) {
    const c = src[i];
    if (comments && c === "/" && src[i + 1] === "/") { while (i < n && src[i] !== "\n") { out += " "; i++; } }
    else if (comments && c === "/" && src[i + 1] === "*") { out += "  "; i += 2; while (i < n && !(src[i] === "*" && src[i + 1] === "/")) { out += blank(src[i]); i++; } out += "  "; i += 2; }
    else if (strings && (c === '"' || c === "`" || c === "'")) {
      const q = c; out += " "; i++;
      while (i < n && src[i] !== q) { if (src[i] === "\\" && q !== "`") { out += "  "; i += 2; continue; } out += blank(src[i]); i++; }
      out += i < n ? " " : ""; i++;
    } else { out += c; i++; }
  }
  return out;
}
const lineOf = (src, off) => src.slice(0, off).split("\n").length;

// ---------- generic Go file analysis ----------
const internalImportRe = /"livecommerce\/(internal\/[A-Za-z0-9_./-]+)"/g;

function parseGoFile(rel) {
  const raw = read(rel);
  const code = mask(raw, { strings: false }); // comments stripped, strings kept (patterns, consts, URLs)
  const bare = mask(raw);                      // comments + strings stripped (identifier scanning)
  const imports = new Map(); // alias -> internal pkg dir
  for (const m of raw.matchAll(/import\s+(?:(\w+|\.|_)\s+)?"(livecommerce\/internal\/[A-Za-z0-9_./-]+)"/g)) {
    const p = m[2].slice("livecommerce/".length);
    imports.set(m[1] && m[1] !== "." && m[1] !== "_" ? m[1] : p.split("/").pop(), p);
  }
  for (const m of raw.matchAll(/import\s*\(([\s\S]*?)\n\)/g)) {
    for (const b of m[1].matchAll(/(?:(\w+|\.|_)\s+)?"(livecommerce\/internal\/[A-Za-z0-9_./-]+)"/g)) {
      const p = b[2].slice("livecommerce/".length);
      imports.set(b[1] && b[1] !== "." && b[1] !== "_" ? b[1] : p.split("/").pop(), p);
    }
  }
  // top-level funcs: {name, recvType, sigEnd (offset of '{'), start, end}; generics (route[T any]) allowed
  const funcs = [];
  for (const m of bare.matchAll(/^func\s+(?:\(\s*(\w+)\s+\*?(\w+)\s*\)\s*)?(\w+)(?:\[[^\]]*\])?\s*\(/gm)) {
    const name = m[3], recvType = m[2] || null;
    // find the body brace: first '{' at depth 0 after the signature close paren
    let i = m.index + m[0].length - 1, depth = 0, sigEnd = -1;
    const parenOpen = i;
    for (; i < bare.length; i++) {
      if (bare[i] === "(") depth++;
      else if (bare[i] === ")") { depth--; if (depth === 0) { sigEnd = i; break; } }
    }
    let j = bare.indexOf("{", sigEnd);
    if (j < 0) continue;
    depth = 0;
    let end = -1;
    for (let k = j; k < bare.length; k++) {
      if (bare[k] === "{") depth++;
      else if (bare[k] === "}") { depth--; if (depth === 0) { end = k; break; } }
    }
    funcs.push({ name, recvType, start: m.index, parenOpen, sigEnd, bodyStart: j, end, sig: code.slice(m.index, sigEnd + 1) });
  }
  // string consts/vars (file scope): name = "lit"
  const consts = new Map();
  for (const m of code.matchAll(/(?:^|\n)\s*(?:const|var)?\s*(\w+)\s*(?::?[A-Za-z0-9_.*[\]{}<>| ]*?)=\s*"([^"\n]*)"/g)) {
    if (!consts.has(m[1])) consts.set(m[1], m[2]);
  }
  return { rel, raw, code, bare, imports, funcs, consts };
}

// identifier -> package attribution inside a text span
function refsIn(text, imports) {
  const pkgs = new Set();
  for (const [alias, p] of imports) if (new RegExp(`\\b${alias}\\.`).test(text)) pkgs.add(p);
  return pkgs;
}
// name -> package maps from signature params and local bindings (receiver-style calls like cs.Comments / console.Read)
function bindMap(text, imports) {
  const map = new Map();
  for (const m of text.matchAll(/(\w+(?:\s*,\s*\w+)*)\s+\*?(?:\[\w+\s*\]\s*)?(\w+)\.[A-Za-z_]\w*(?:\s*[,\)]|\s*$)/gm)) {
    for (const name of m[1].split(/\s*,\s*/)) if (imports.has(m[2])) map.set(name, imports.get(m[2]));
  }
  for (const m of text.matchAll(/(\w+)\s*(?:,\s*\w+\s*)?:=\s*&?(\w+)\.\w/g)) if (imports.has(m[2])) map.set(m[1], imports.get(m[2]));
  for (const m of text.matchAll(/var\s+(\w+)\s+\*?(\w+)\.\w/g)) if (imports.has(m[2])) map.set(m[1], imports.get(m[2]));
  return map;
}
function receiverRefs(text, binds) {
  const pkgs = new Set();
  for (const [name, p] of binds) if (new RegExp(`\\b${name}\\.`).test(text)) pkgs.add(p);
  return pkgs;
}

// ---------- package-indexed Go dirs with function-level analysis ----------
function indexPkg(dir) {
  const files = readdirSync(path.join(root, dir)).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go")).sort()
    .map((f) => parseGoFile(`${dir}/${f}`));
  const byFunc = new Map(); // func name (any receiver) -> [ {file, fn} ]
  for (const file of files) for (const fn of file.funcs) {
    if (!byFunc.has(fn.name)) byFunc.set(fn.name, []);
    byFunc.get(fn.name).push({ file, fn });
  }
  return { dir, files, byFunc };
}
// function-level package attribution: alias refs (sig+body) + param/binding receivers + same-pkg callee recursion
function analyzeFunc(pkgIdx, file, fn, depth, memo) {
  const key = `${file.rel}#${fn.name}@${fn.start}`;
  if (memo.has(key)) return memo.get(key);
  const acc = new Set();
  memo.set(key, acc); // cycles -> partial (fine: union only grows)
  if (depth <= 0) return acc;
  const text = file.bare.slice(fn.start, fn.end + 1);
  for (const p of refsIn(text, file.imports)) acc.add(p);
  for (const p of receiverRefs(text, bindMap(file.code.slice(fn.start, fn.end + 1), file.imports))) acc.add(p);
  for (const m of text.matchAll(/(?:\bh\.|\bcs\.|\be\.)?(\w+)\s*\(/g)) {
    const name = m[1];
    if (["if", "for", "switch", "return", "func", "go", "defer", "range", "case", "make", "new", "len", "cap", "append", "copy", "delete", "panic", "recover", "print", "println", "error", "any", "string", "int", "int64", "bool", "byte", "rune", "float64", "errorf"].includes(name)) continue;
    for (const target of pkgIdx.byFunc.get(name) ?? []) {
      if (target.file === file && target.fn === fn) continue;
      for (const p of analyzeFunc(pkgIdx, target.file, target.fn, depth - 1, memo)) acc.add(p);
    }
  }
  return acc;
}

// ---------- route table: httpapi + identityhttp (mux.HandleFunc patterns) ----------
function callSpan(bare, openParen) {
  let depth = 0;
  for (let i = openParen; i < bare.length; i++) {
    if (bare[i] === "(") depth++;
    else if (bare[i] === ")") { depth--; if (depth === 0) return [openParen, i]; }
  }
  return [openParen, bare.length];
}
function splitTop(text, sep) {
  const parts = []; let depth = 0, cur = "", q = null;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (q) { cur += c; if (c === "\\" && q !== "`") { cur += text[++i] ?? ""; } else if (c === q) q = null; continue; }
    if (c === '"' || c === "`" || c === "'") { q = c; cur += c; continue; }
    if ("([{".includes(c)) depth++;
    if (")]}".includes(c)) depth--;
    if (c === sep && depth === 0) { parts.push(cur); cur = ""; continue; }
    cur += c;
  }
  parts.push(cur);
  return parts;
}
// string-aware brace-balanced close of the '{' at index i of a code view; -1 when unbalanced
function balancedFrom(code, i) {
  let depth = 0, q = null;
  for (let k = i; k < code.length; k++) {
    const c = code[k];
    if (q) { if (c === "\\") k++; else if (c === q) q = null; continue; }
    if (c === '"' || c === "`" || c === "'") { q = c; continue; }
    if (c === "{") depth++;
    else if (c === "}") { depth--; if (depth === 0) return k; }
  }
  return -1;
}
// single-line name := expr / const name = expr bindings (multi-assign a, b := x, y paired positionally)
function collectBinds(text, into) {
  for (const m of text.matchAll(/(?:^|\n)[ \t]*(?:const[ \t]+|var[ \t]+)?([A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)(?:\s+[\w.*\[\]{}<>|]+?)?\s*(?::=|=(?!=))\s*([^\n]+)/g)) {
    const names = m[1].split(/\s*,\s*/);
    const rhss = splitTop(m[2], ",");
    if (names.length === rhss.length) names.forEach((n, i) => { const v = rhss[i].trim(); if (v && !into.has(n)) into.set(n, v); });
    else if (names.length === 1) { const v = m[2].trim(); if (v && !into.has(names[0])) into.set(names[0], v); }
  }
  return into;
}

const routeDiag = [];
// Expression evaluation to string values: literal terms, '+' concatenation, identifier resolution through layered
// contexts (range overrides -> call-site-resolved string params -> func-local binds -> package-scope binds/consts).
// Returns string[] (multiple values = alternates, e.g. a `path` param fed by six call sites) or null when unresolved.
function makeEval(pkgCtxCache) {
  const evalExprs = (expr, ctx, depth = 8) => {
    if (depth < 0) return null;
    let combos = [""];
    for (const term of splitTop(expr, "+")) {
      const t = term.trim();
      if (!t) continue;
      let vals = null;
      const lit = /^"((?:[^"\\]|\\.)*)"$/s.exec(t) || /^`([^`]*)`$/s.exec(t);
      if (lit) vals = [lit[1]];
      else if (/^\w+$/.test(t) || /^\w+\.\w+$/.test(t)) vals = resolveIdent(t, ctx, depth);
      if (!vals) return null;
      const next = [];
      for (const c of combos) for (const v of vals) if (next.length < 64) next.push(c + v);
      combos = next;
    }
    return combos;
  };
  const resolveIdent = (name, ctx, depth) => {
    if (ctx.over?.has(name)) { const v = ctx.over.get(name); return v === null || v === undefined ? null : [v]; }
    const p = ctx.params?.get(name);
    if (p?.length) return p;
    for (const map of [ctx.fnBinds, ctx.pkgBinds]) {
      const b = map?.get(name);
      if (b !== undefined) { const v = evalExprs(b, ctx, depth - 1); if (v) return v; }
    }
    const c = ctx.pkgConsts?.get(name) ?? ctx.fileConsts?.get(name);
    if (c !== undefined) return [c];
    return null;
  };
  return evalExprs;
}

function buildHandleFuncRoutes(pkgIdx, regPkg) {
  const routes = [];
  const memo = new Map();
  const evalExprs = makeEval();
  // package-scope consts + expression binds are visible in every file of the package (customerBase, accountBase, ...);
  // func-local binds are collected per func only (two funcs may bind `base` differently — no cross-func leakage).
  const pkgConsts = new Map();
  for (const file of pkgIdx.files) for (const [k, v] of file.consts) if (!pkgConsts.has(k)) pkgConsts.set(k, v);
  const pkgBinds = new Map();
  for (const file of pkgIdx.files) {
    let scoped = file.code; // blank out func bodies to keep package scope only
    for (const fn of [...file.funcs].sort((a, b) => b.start - a.start)) scoped = scoped.slice(0, fn.bodyStart) + " ".repeat(Math.max(0, fn.end + 1 - fn.bodyStart)) + scoped.slice(fn.end + 1);
    collectBinds(scoped, pkgBinds);
  }
  // string-typed params resolved from call sites inside the package (registerBlocklistRoutes(mux, pool, base) <- base
  // literal; route(mux, "list"|"invite"|..., fn) -> alternates). Recursive through caller params, depth-capped.
  const paramMemo = new Map();
  const resolveStringParams = (file, fn, depth) => {
    const key = `${file.rel}#${fn.name}@${fn.start}`;
    if (paramMemo.has(key)) return paramMemo.get(key);
    const out = new Map();
    paramMemo.set(key, out);
    if (depth <= 0) return out;
    // positional param names with string type (receiver already excluded from sig params)
    const paramsText = file.code.slice(fn.parenOpen + 1, fn.sigEnd);
    const positions = [];
    for (const part of splitTop(paramsText, ",")) {
      const pm = /^(\w+(?:\s*,\s*\w+)*)\s+(\S.*)$/.exec(part.trim());
      if (!pm) continue;
      for (const n of pm[1].split(/\s*,\s*/)) positions.push({ name: n, isString: /^string$/.test(pm[2].trim()) });
    }
    if (!positions.some((p) => p.isString)) return out;
    const callRe = new RegExp(`\\b${fn.name.replace(/\$/g, "$$")}\\s*(?:\\[[^\\]]*\\])?\\s*\\(`, "g");
    for (const file2 of pkgIdx.files) {
      for (const cm of file2.bare.matchAll(callRe)) {
        if (/\bfunc\s*$/.test(file2.bare.slice(Math.max(0, cm.index - 24), cm.index))) continue; // the definition itself
        const open = cm.index + cm[0].length - 1;
        const [cs2, ce2] = callSpan(file2.bare, open);
        const args = splitTop(file2.code.slice(cs2 + 1, ce2), ",");
        const fn2 = file2.funcs.filter((f) => f.bodyStart <= cs2 && f.end >= ce2).sort((a, b) => b.start - a.start)[0] ?? null;
        const ctx2 = {
          fnBinds: fn2 ? collectBinds(file2.code.slice(fn2.bodyStart, fn2.end + 1), new Map()) : null,
          params: fn2 ? resolveStringParams(file2, fn2, depth - 1) : null,
          pkgBinds, pkgConsts, fileConsts: file2.consts,
        };
        positions.forEach((pos, i) => {
          if (!pos.isString || args[i] === undefined) return;
          const vals = evalExprs(args[i].trim(), ctx2, 4);
          if (!vals) return;
          if (!out.has(pos.name)) out.set(pos.name, []);
          for (const v of vals) if (!out.get(pos.name).includes(v)) out.get(pos.name).push(v);
        });
      }
    }
    return out;
  };
  // range-loop sources: inline []T{...} literals (brace-balanced, strings may hold '{') or named slices (func-local or
  // package binds, plus `name = append(name, ...)` growth, e.g. operations.go's actions)
  const rangeItems = (code, fn, callIdx, varName) => {
    const before = code.slice(fn.bodyStart, callIdx);
    const re = new RegExp(`for\\s+(?:[\\w.]+\\s*,\\s*)?${varName}\\s*:=\\s*range\\s*`, "g");
    let m, last = -1;
    while ((m = re.exec(before))) last = m.index + m[0].length;
    if (last < 0) return null;
    const rest = before.slice(last);
    // items of a composite literal whose '{' is at braceOff in src; []struct{...}{items} carries the ITEMS in the
    // second brace group (the first is the anonymous type body) — a named type ([]page{...}) has no second group.
    const at = (src, braceOff) => {
      let b = braceOff;
      if (/\[\]\s*struct\s*\{$/.test(src.slice(Math.max(0, braceOff - 30), braceOff + 1))) {
        const closeT = balancedFrom(src, b);
        if (closeT > 0) { let k = closeT + 1; while (k < src.length && /\s/.test(src[k])) k++; if (src[k] === "{") b = k; }
      }
      const close = balancedFrom(src, b);
      return close > 0 ? splitTop(src.slice(b + 1, close), ",").map((s) => s.trim()).filter(Boolean) : null;
    };
    if (rest.trimStart().startsWith("[")) {
      const braceAt = last + rest.indexOf("{");
      return braceAt > last ? at(before, braceAt) : null;
    }
    const idm = /^(\w+)/.exec(rest.trimStart());
    if (!idm) return null;
    const defRe = new RegExp(`\\b${idm[1]}\\s*(?::=|=)\\s*\\[?[^\\n{]*\\{`, "g");
    let dm;
    while ((dm = defRe.exec(code))) {
      const items = at(code, dm.index + dm[0].length - 1);
      if (!items) continue;
      for (const am of code.matchAll(new RegExp(`\\b${idm[1]}\\s*=\\s*append\\(\\s*${idm[1]}\\s*,`, "g"))) {
        const open2 = am.index + am[0].length - 1;
        const [a1, a2] = callSpan(code, open2);
        items.push(...splitTop(code.slice(a1 + 1, a2), ",").slice(1).map((s) => s.trim()).filter(Boolean));
      }
      return items;
    }
    return null;
  };
  // content of an item's brace group (ledgerAction{"query", ...} / {"products", false, func...}) or the item itself
  const groupContent = (item) => {
    const b = item.indexOf("{");
    if (b < 0) return item;
    const c = balancedFrom(item, b);
    return c > 0 ? item.slice(b + 1, c) : item;
  };
  const itemValues = (item, field, ctx) => {
    if (field) {
      const content = groupContent(item);
      const named = new RegExp(`\\b${field}:\\s*("(?:[^"\\\\]|\\\\.)*"|[\\w.]+)`).exec(content);
      if (named) { const l = /^"((?:[^"\\]|\\.)*)"$/.exec(named[1]); return l ? [l[1]] : evalExprs(named[1], ctx); }
      const first = splitTop(content, ",")[0]?.trim() ?? "";
      const l = /^"((?:[^"\\]|\\.)*)"$/.exec(first);
      if (l) return [l[1]];
      return evalExprs(first, ctx);
    }
    return evalExprs(item, ctx);
  };

  for (const file of pkgIdx.files) {
    for (const m of file.bare.matchAll(/\.Handle(?:Func)?\s*\(/g)) {
      const open = m.index + m[0].length - 1;
      const [cs, ce] = callSpan(file.bare, open);
      const argsCode = file.code.slice(cs + 1, ce);
      const args = splitTop(argsCode, ",");
      const expr = (args[0] ?? "").trim();
      const fn = file.funcs.filter((f) => f.bodyStart <= cs && f.end >= ce).sort((a, b) => b.start - a.start)[0] ?? null;
      const ctx = {
        fnBinds: fn ? collectBinds(file.code.slice(fn.bodyStart, fn.end + 1), new Map()) : null,
        params: fn ? resolveStringParams(file, fn, 3) : null,
        pkgBinds, pkgConsts, fileConsts: file.consts,
      };
      const where = `${file.rel}:${lineOf(file.raw, cs)}`;
      let patterns = evalExprs(expr, ctx);
      // unresolved terms that are range-loop variables (pattern, path, suffix, m, p.slug, mode.path, a.name):
      // per-item alternates; non-string items (http.Method*) substitute "" (method position — matching ignores method)
      if (!patterns && fn) {
        for (const term of splitTop(expr, "+")) {
          const t = term.trim();
          const idm = /^(\w+)(?:\.(\w+))?$/.exec(t);
          if (!idm || evalExprs(t, ctx)) continue;
          const items = rangeItems(file.code, fn, cs, idm[1]);
          if (!items) continue;
          const vals = [];
          for (const item of items) for (const v of itemValues(item, idm[2], ctx) ?? []) if (!vals.includes(v)) vals.push(v);
          const over = new Map([[idm[2] ? `${idm[1]}.${idm[2]}` : idm[1], null]]);
          if (vals.length) {
            patterns = [];
            for (const v of vals) {
              over.set(idm[2] ? `${idm[1]}.${idm[2]}` : idm[1], v);
              for (const p of evalExprs(expr, { ...ctx, over }) ?? []) if (!patterns.includes(p)) patterns.push(p);
            }
          } else {
            over.set(idm[1], ""); // method-variable range (for _, m := range []string{http.MethodPost, ...})
            patterns = evalExprs(expr, { ...ctx, over });
          }
          break;
        }
      }
      if (!patterns) { routeDiag.push(`${where}: unresolved pattern ${expr.slice(0, 80)}`); continue; }
      // attribution: the registration STATEMENT only (helper call args included) on the bare view + the param/local
      // bindings the statement itself names (foundation -> platform, cs -> live). Deliberately not every binding of the
      // enclosing func: NewHandler constructs every service, so func-wide receiver attribution re-created round 1's
      // explosion on the routes registered in its own body (/v1/admin/stores, /healthz, audit-events).
      const stmt = file.bare.slice(cs, ce + 1);
      const pkgs = refsIn(stmt, file.imports);
      const scope = fn ? file.code.slice(fn.start, fn.end + 1) : "";
      const binds = bindMap(scope, file.imports);
      for (const im of stmt.matchAll(/\b(\w+)\b/g)) { const bp = binds.get(im[1]); if (bp) pkgs.add(bp); }
      // helper calls in the statement -> function-level analysis within the same package
      for (const hm of stmt.matchAll(/(\w+)\s*\(/g)) {
        for (const target of pkgIdx.byFunc.get(hm[1]) ?? []) {
          if (target.fn.name === fn?.name && target.file === file) continue;
          for (const p of analyzeFunc(pkgIdx, target.file, target.fn, 3, memo)) pkgs.add(p);
        }
      }
      for (const raw of patterns) {
        const pat = raw.trim();
        const mm = /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(\/.*)$/.exec(pat);
        const method = mm ? mm[1] : null, urlPattern = mm ? mm[2] : pat;
        if (!urlPattern.startsWith("/")) continue;
        routes.push({ method, pattern: urlPattern, regPkg, file: file.rel, line: lineOf(file.raw, cs), pkgs: [...pkgs].sort() });
      }
    }
  }
  return routes;
}

// ---------- route table: buyerhttp kind dispatch ----------
function buildBuyerRoutes(buyerIdx) {
  const handlerFile = buyerIdx.files.find((f) => f.rel.endsWith("handler.go"));
  // routeKind constant names
  const kinds = new Set();
  for (const file of buyerIdx.files) for (const m of file.code.matchAll(/\bkind:\s*(\w+)/g)) kinds.add(m[1]);
  for (const m of handlerFile.code.matchAll(/(?:^|\n)\s*(\w+)\s*(?:routeKind)?\s*(?:=|,)\s*(?:iota)?/g)) if (/Route$|route[A-Z]/.test(m[1])) kinds.add(m[1]);
  // path constants
  const pathConsts = new Map();
  for (const file of buyerIdx.files) for (const [k, v] of file.consts) if (v.startsWith("/v1/buyer")) pathConsts.set(k, v);
  const routes = []; // {pattern, prefix:boolean, kind, file, line}
  const matcherFiles = buyerIdx.files.filter((f) => /func match\w*Route|func matchCatalogV2/.test(f.bare));
  for (const file of matcherFiles) {
    for (const fn of file.funcs.filter((f) => /^match/.test(f.name))) {
      const bodyCode = file.code.slice(fn.bodyStart, fn.end + 1);
      const bodyBare = file.bare.slice(fn.bodyStart, fn.end + 1);
      // case literals / case const idents -> kind on the following return
      const caseRe = /case\s+((?:"[^"\n]*"|\w+)\s*(?:,\s*(?:"[^"\n]*"|\w+))*)\s*:/g;
      for (const cm of bodyCode.matchAll(caseRe)) {
        const after = bodyCode.slice(cm.index + cm[0].length, cm.index + cm[0].length + 300);
        const kind = /(?:kind:\s*(\w+)|return\s+route\{kind:\s*(\w+)|route\{kind:\s*(\w+))/.exec(after);
        if (!kind) continue;
        for (const term of cm[1].split(",").map((s) => s.trim())) {
          const lit = /^"([^"]*)"$/.exec(term);
          const val = lit ? lit[1] : pathConsts.get(term);
          if (!val || !val.startsWith("/v1/buyer")) continue;
          const isV2 = !lit && term === "rest"; // not used; v2 handled below
          routes.push({ pattern: val, exact: true, kind: (kind[1] || kind[2] || kind[3]), file: file.rel, line: lineOf(file.raw, fn.bodyStart + cm.index) });
        }
      }
      // rest == "literal" (matchCatalogV2 style): pattern = v2Prefix + literal
      const v2 = pathConsts.get("v2Prefix") ?? "/v1/buyer/catalog/v2/";
      for (const cm of bodyCode.matchAll(/rest\s*==\s*"([^"]+)"/g)) {
        const after = bodyCode.slice(cm.index, cm.index + 200);
        const kind = /kind:\s*(\w+)/.exec(after);
        if (kind) routes.push({ pattern: v2 + cm[1], exact: true, kind: kind[1], file: file.rel, line: lineOf(file.raw, fn.bodyStart + cm.index) });
      }
      for (const cm of bodyCode.matchAll(/CutPrefix\(rest,\s*"([^"]+)"\)/g)) {
        const after = bodyCode.slice(cm.index, cm.index + 300);
        const kind = /kind:\s*(\w+)/.exec(after);
        if (kind) routes.push({ pattern: v2 + cm[1] + "{*}", exact: false, kind: kind[1], file: file.rel, line: lineOf(file.raw, fn.bodyStart + cm.index) });
      }
      // CutPrefix(path, "literal") -> prefix route; kind from the following block (may hold a sub-switch: collect all kinds)
      for (const cm of bodyCode.matchAll(/CutPrefix\(path,\s*(?:"([^"]+)"|(\w+))\)/g)) {
        const val = cm[1] ?? pathConsts.get(cm[2] ?? "");
        if (!val || !val.startsWith("/v1/buyer")) continue;
        const after = bodyCode.slice(cm.index, cm.index + 700);
        const ks = [...after.matchAll(/kind:\s*(\w+)|\{\s*"([^"]*)",\s*(\w+)\s*\}/g)];
        if (!ks.length) continue;
        // suffix tables: {"/payment/prepare", paymentPrepareRoute} inside CutPrefix(path, "/v1/buyer/orders/")
        for (const k of ks) {
          if (k[1]) routes.push({ pattern: val + "{*}", exact: false, kind: k[1], file: file.rel, line: lineOf(file.raw, fn.bodyStart + cm.index) });
          else if (k[2]?.startsWith("/")) routes.push({ pattern: val + "{*}" + k[2], exact: false, kind: k[3], file: file.rel, line: lineOf(file.raw, fn.bodyStart + cm.index) });
        }
      }
      // prefix tables: {{"/v1/buyer/quotes/", quoteRoute}, ...}
      for (const cm of bodyCode.matchAll(/\{\s*"(\/v1\/buyer\/[^"]+)",\s*(\w+)\s*\}/g)) {
        routes.push({ pattern: cm[1] + "{*}", exact: false, kind: cm[2], file: file.rel, line: lineOf(file.raw, fn.bodyStart + cm.index) });
      }
    }
  }
  // kind -> serve attribution: every `case <kinds>:` block and `selected.kind == <kind>` block in any buyerhttp file
  const kindPkgs = new Map(); // kind -> Set(pkg)
  const kindSite = new Map(); // kind -> file:line (dispatch evidence)
  const memo = new Map();
  const addKind = (kind, pkgs, file, off) => {
    if (!kindPkgs.has(kind)) kindPkgs.set(kind, new Set());
    for (const p of pkgs) kindPkgs.get(kind).add(p);
    if (!kindSite.has(kind)) kindSite.set(kind, `${file.rel}:${lineOf(file.raw, off)}`);
  };
  for (const file of buyerIdx.files) {
    const binds = bindMap(file.code, file.imports);
    for (const cm of file.bare.matchAll(/case\s+((?:\w+\s*,\s*)*\w+)\s*:/g)) {
      const list = cm[1].split(",").map((s) => s.trim()).filter((s) => kinds.has(s));
      if (!list.length) continue;
      // block text until the next top-level `case ` or `}` at the same indent: approximate with next case/default/end of switch
      const rest = file.bare.slice(cm.index + cm[0].length);
      const stop = rest.search(/\n\s*(?:case\s|default\s*:|\})/);
      const block = rest.slice(0, stop < 0 ? 2000 : stop);
      const pkgs = refsIn(block, file.imports);
      for (const p of receiverRefs(block, binds)) pkgs.add(p);
      for (const hm of block.matchAll(/(\w+)\s*\(/g)) for (const target of file.funcs.filter((f) => f.name === hm[1])) for (const p of analyzeFunc(buyerIdx, file, target, 2, memo)) pkgs.add(p);
      for (const mm of block.matchAll(/\bh\.(\w+)\s*\(/g)) for (const target of buyerIdx.byFunc.get(mm[1]) ?? []) for (const p of analyzeFunc(buyerIdx, target.file, target.fn, 2, memo)) pkgs.add(p);
      for (const kind of list) addKind(kind, pkgs, file, cm.index);
    }
    for (const cm of file.bare.matchAll(/\.kind\s*==\s*(\w+)/g)) {
      if (!kinds.has(cm[1])) continue;
      const rest = file.bare.slice(cm.index);
      const stop = rest.search(/\n\s*\}\s*\n/);
      const block = rest.slice(0, stop < 0 ? 2500 : stop);
      const pkgs = refsIn(block, file.imports);
      for (const p of receiverRefs(block, binds)) pkgs.add(p);
      for (const mm of block.matchAll(/\bh\.(\w+)\s*\(/g)) for (const target of buyerIdx.byFunc.get(mm[1]) ?? []) for (const p of analyzeFunc(buyerIdx, target.file, target.fn, 2, memo)) pkgs.add(p);
      addKind(cm[1], pkgs, file, cm.index);
    }
  }
  // fallback: a kind no dispatch block named gets the handler file's direct service imports (conservative within buyerhttp)
  const fallback = refsIn(handlerFile.bare, handlerFile.imports);
  return routes.map((r) => ({
    ...r,
    regPkg: "internal/buyerhttp",
    pkgs: [...new Set(["internal/buyerhttp", ...(kindPkgs.get(r.kind) ?? fallback)])].sort(),
    dispatch: kindSite.get(r.kind) ?? `${handlerFile.rel}:0`,
  }));
}

// ---------- pattern matching (Go 1.22 {param} vs URL templates {*}) ----------
function segSplit(p) { return p.replace(/^\//, "").split("/").filter(Boolean); }
function segEq(r, u) { // route segment vs url segment
  if (r === "{*}") return true;                       // normalized wildcard absorbs anything (and everything after)
  if (u === "{*}") return true;                       // URL template segment (${...}, uuid, digits) matches anything
  if (r.startsWith("{") && r.endsWith("}")) return true; // Go 1.22 {param}
  if (u.endsWith("*")) return r.startsWith(u.slice(0, -1)); // playwright glob tail: messages*
  return r === u;
}
function patternMatches(routePat, url, urlPrefix) {
  const rp = segSplit(routePat), us = segSplit(url);
  for (let i = 0; i < us.length; i++) {
    if (rp[i] === undefined) return false;
    if (rp[i] === "{*}") return true;                 // route-side normalized wildcard absorbs the rest of the URL
    if (!segEq(rp[i], us[i])) return false;
    if (i === us.length - 1 && us[i] === "{*}") return true; // URL-side template tail (${path}) absorbs deeper routes
    if (i === us.length - 1 && urlPrefix && us[i].endsWith("*")) return true; // glob tail: route may continue deeper
  }
  if (!urlPrefix && rp.length !== us.length) return false;
  if (urlPrefix && rp.length < us.length) return false;
  return true;
}
// fragment (Go-side partial path like "/shipments/tracking-import/") -> routes containing it as a contiguous tail-ish subsequence
function fragmentMatches(routePat, frag) {
  const rp = segSplit(routePat), fs = segSplit(frag);
  if (fs.length < 2 || !fs.some((s) => s !== "{*}" && !s.includes("*"))) return false;
  outer: for (let start = 0; start + fs.length <= rp.length; start++) {
    for (let i = 0; i < fs.length; i++) if (!segEq(rp[start + i], fs[i])) continue outer;
    return true;
  }
  return false;
}

// ---------- BFF table ----------
// Entry literals are the /v1/ Go targets a BFF route file (plus its one-level lib imports) actually names. Catch-all
// pass-through entries ([...resource]/[...path]) contribute their literals only as a LAST-resort tier: the admin
// catch-all imports ~20 request-grammar libs whose literals span every admin resource, and charging all of them to
// every /api/stores/... URL was round 2's first over-attribution bug. Literals that normalize to a trailing "{*}"
// (pure pass-through templates like `${config.api}/v1/buyer/${path}`) carry no information for catch-alls and are
// dropped there; for a specific route file (e.g. lib/auth.ts `/v1/identity/${path}`) they are kept: the wildcard then
// legitimately absorbs every route below the file's fixed prefix.
function bffTable() {
  const table = [];
  // Files whose /v1/ literals speak for one route handler: the route file plus its imports, followed transitively
  // (depth 2): password routes are route.ts -> ../step1 -> @/lib/auth, and only lib/auth holds the /v1/identity/...
  // target. Relative sibling imports resolve against the importing file's dir; @/ against the app root (+ src/).
  const closure = (rel, app, depth, visited) => {
    visited.add(rel);
    if (depth <= 0) return;
    let raw; try { raw = read(rel); } catch { return; }
    for (const m of raw.matchAll(/from\s+"((?:@\/|\.{1,2}\/)[\w./@-]+)"/g)) {
      const spec = m[1];
      const cands = spec.startsWith("@/")
        ? [`apps/${app}/${spec.slice(2)}.ts`, `apps/${app}/${spec.slice(2)}/index.ts`, `apps/${app}/src/${spec.slice(2)}.ts`, `apps/${app}/src/${spec.slice(2)}/index.ts`]
        : [`${path.posix.join(path.posix.dirname(rel.split(path.sep).join("/")), spec)}.ts`, `${path.posix.join(path.posix.dirname(rel.split(path.sep).join("/")), spec)}/index.ts`];
      for (const c of cands) if (!visited.has(c) && existsSync(path.join(root, c))) { closure(c, app, depth - 1, visited); break; }
    }
  };
  for (const app of ["admin", "storefront"]) {
    const appDir = path.join(root, "apps", app, "app");
    const walk = (d) => {
      for (const ent of readdirSync(d, { withFileTypes: true })) {
        const p = path.join(d, ent.name);
        if (ent.isDirectory()) walk(p);
        else if (ent.name === "route.ts") {
          const rel = path.relative(root, p);
          const segs = path.relative(appDir, path.dirname(p)).split(path.sep).filter((s) => s !== "(public)");
          const url = "/" + segs.map((s) => s.replace(/^\[[^\]]*\]$/, "{*}")).join("/");
          const files = new Set();
          closure(rel, app, 2, files);
          if (rel.includes("api/buyer/[...path]")) files.add(`apps/${app}/lib/buyer-server.ts`);
          const catchAll = /\[\.\.\./.test(rel);
          const lits = new Set();
          for (const f of files) {
            const t = read(f);
            for (const lm of t.matchAll(/["'`]([^"'`\s]*\/v1\/[^"'`\s]*)["'`]/g)) {
              const norm = normalizeUrl(lm[1].slice(lm[1].indexOf("/v1/")));
              if (catchAll && norm.endsWith("{*}")) continue;
              lits.add(norm);
            }
          }
          table.push({ app, url: url.replace(/\/$/, "") || "/", file: rel, catchAll, lits: [...lits] });
        }
      }
    };
    walk(appDir);
  }
  return table;
}

// ---------- URL extraction + normalization ----------
const UUID = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
function normalizeUrl(u) {
  const rooted = u.startsWith("/");
  let s = u.replace(/^[a-z][a-z0-9+.-]*:\/\/[^/\s"'`]*/, ""); // strip scheme://origin
  s = s.replace(/\?.*$/, "").replace(/#.*$/, "");
  s = s.replace(/^\*\*\//, "/");
  if (!s.startsWith("/")) s = "/" + s;
  let segs = segSplit(s).map((seg) => {
    if (seg.includes("$")) return "{*}";
    if (new RegExp(`^${UUID}$`).test(seg)) return "{*}";
    if (/^\d+$/.test(seg)) return "{*}";
    if (seg.endsWith("*")) return seg; // playwright glob tail stays for prefix matching
    return seg;
  });
  // Drop leading template/host junk (`${origin}/api/...`) only when the literal was not already a rooted path: a
  // rooted `/internal/v1/...` is a harness-internal mock callback, not our API surface, and must not be re-rooted.
  const at = segs.findIndex((s2) => ["api", "v1", "bff", "__test"].includes(s2));
  if (at > 0 && (!rooted || segs.slice(0, at).some((x) => x.includes("{*}")))) segs = segs.slice(at);
  return "/" + segs.join("/");
}
function extractUrls(text) {
  const urls = new Set();
  for (const m of text.matchAll(/["'`]([^"'`\s]*\/(?:api|v1|bff|__test)\/[^"'`\s]*)["'`]/g)) {
    // skip absolute URLs aimed at external hosts (sandbox PSP APIs etc.) — they are not routes of this system
    const sch = /^[a-z][a-z0-9+.-]*:\/\/([^/\s"'`]*)/i.exec(m[1]);
    if (sch && !/^(?:localhost(?::\d+)?|127\.0\.0\.1(?::\d+)?|\[?::1\]?(?::\d+)?|\$\{)/i.test(sch[1])) continue;
    urls.add(m[1]);
  }
  return [...urls].map(normalizeUrl).filter((u) => /^\/(api|v1|bff|__test)\//.test(u));
}
// Go-side partial path literals ("/shipments/tracking-import/") used with strings.Contains/HasPrefix against r.URL.Path
function extractGoFragments(text) {
  const frags = new Set();
  for (const m of text.matchAll(/"(\/[a-z0-9][^"\s]*)"/g)) {
    const v = m[1];
    if (/\.(?:go|ts|tsx|mjs|js|json|sh|sql|png|jpe?g|log|md|html|css|svg|txt|zip|pem|crt|key)$/i.test(v)) continue;
    if (/^\/(?:tmp|dev|usr|etc|var|proc|sys|Volumes|output)\b/.test(v)) continue;
    const segs = segSplit(v);
    if (segs.length < 2) continue;
    if (/^(v1|api|bff|__test)$/.test(segs[0])) continue; // full URLs go through extractUrls
    frags.add(normalizeUrl(v));
  }
  return [...frags];
}

// ---------- tests/foundation + cmd indexes ----------
const tfDir = path.join(root, "tests/foundation");
const tfFiles = readdirSync(tfDir).filter((f) => f.endsWith(".go")).sort();
const tf = new Map();
for (const f of tfFiles) {
  const raw = readFileSync(path.join(tfDir, f), "utf8");
  const bare = mask(raw);
  const imports = new Set([...raw.matchAll(internalImportRe)].map((m) => m[1]));
  const cmdRefs = new Set([...raw.matchAll(/["'](?:\.\.?\/)*cmd\/([a-z0-9-]+)["']/g)].map((m) => `cmd/${m[1]}`));
  const header = raw.split("\n").slice(0, 16).join("\n");
  const specs = new Set([...raw.matchAll(/((?:tests|apps)\/[A-Za-z0-9_.@/-]+\.(?:spec\.ts|mjs|test\.ts|test\.mjs))/g)].map((m) => m[1]));
  // bare spec/runner file names handed to shared helpers (brfPlaywright(t, ..., []string{"tracking-backfill.spec.ts"}))
  const bareSpecs = new Set();
  for (const m of raw.matchAll(/"([A-Za-z0-9_.@-]+\.(?:spec\.ts|mjs))"/g)) {
    for (const dir of ["tests/admin", "tests/storefront", "tests/e2e", "tests/ui", "apps/admin/tests", "apps/storefront/tests"]) {
      if (existsSync(path.join(root, dir, m[1]))) { bareSpecs.add(`${dir}/${m[1]}`); break; }
    }
  }
  const suites = new Set([...raw.matchAll(/LC_BROWSER_SUITE"?\s*(?::|=)\s*\\?"?([a-z0-9-]+)/g)].map((m) => m[1]));
  const urls = extractUrls(raw);
  const fragments = extractGoFragments(raw);
  tf.set(f, { raw, bare, imports, cmdRefs, header, specs, bareSpecs, suites, urls, fragments });
}
const cmdFiles = execFileSync("git", ["ls-files", "-z", "--", "cmd/"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 })
  .split("\0").filter(Boolean).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"));
const cmdImports = new Map();
for (const f of cmdFiles) {
  const d = path.posix.dirname(f);
  if (!cmdImports.has(d)) cmdImports.set(d, new Map());
  const raw = read(f);
  for (const m of raw.matchAll(internalImportRe)) {
    const line = lineOf(raw, m.index);
    if (!cmdImports.get(d).has(m[1])) cmdImports.get(d).set(m[1], `${f}:${line}`);
  }
}

// internal package dirs (tracked non-test Go files)
const pkgDirs = [...new Set(execFileSync("git", ["ls-files", "-z", "--", "internal/"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 })
  .split("\0").filter(Boolean).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go")).map((f) => path.posix.dirname(f)))].sort();
const isPkg = (p) => pkgDirs.includes(p);

// playwright suite map
const pwConfig = read("playwright.config.ts");
const suites = {};
for (const m of pwConfig.matchAll(/"([a-z0-9-]+)":\s*\[([^\]]*)\]/g)) suites[m[1]] = [...m[2].matchAll(/"([^"]+\.(?:spec|test)\.ts)"/g)].map((x) => `tests/admin/${x[1]}`);

// ---------- route tables ----------
const httpapiIdx = indexPkg("internal/httpapi");
const identityIdx = indexPkg("internal/identityhttp");
const buyerIdx = indexPkg("internal/buyerhttp");
const handleRoutes = [...buildHandleFuncRoutes(httpapiIdx, "internal/httpapi"), ...buildHandleFuncRoutes(identityIdx, "internal/identityhttp")];
const buyerRoutes = buildBuyerRoutes(buyerIdx);
const allRoutes = [...handleRoutes, ...buyerRoutes];
const bff = bffTable();

// Staged Go-path candidates for one observed spec/harness URL. Each tier is tried in order; the first tier whose
// candidates match real route registrations wins (so the admin catch-all's lib literals can never outvote the
// callBackend pass-through rule). Tiers:
//   1 direct /v1/ URLs (Go harness + specs that hold the API origin)
//   2 BFF pass-through suffixes: /api/stores/<store>/REST -> /v1/admin/stores/<store>/REST (apps/admin/lib/backend.ts
//     callBackend); /api/buyer/REST -> /v1/buyer/REST (apps/storefront/lib/buyer-server.ts line ~849)
//   3 the most specific non-catch-all apps/*/app/**/route.ts entry matching the URL: its /v1/ literals (+1-level libs)
//   4 catch-all BFF entry literals (transforming handlers like buyer-server's session/prepare -> /v1/buyer/session)
function bffEntriesFor(url) {
  const segs = segSplit(url);
  const hits = [];
  for (const ent of bff) {
    const ep = segSplit(ent.url);
    if (ep.length > segs.length && !(ent.catchAll && ep.length <= segs.length)) continue;
    if (!ent.catchAll && ep.length !== segs.length) continue;
    let ok = true;
    for (let i = 0; i < Math.min(ep.length, segs.length); i++) if (!segEq(ep[i], segs[i]) && ep[i] !== segs[i]) { ok = false; break; }
    if (ok) hits.push({ ent, statics: ep.filter((s) => s !== "{*}").length });
  }
  return hits.sort((a, b) => (b.ent.catchAll === a.ent.catchAll ? b.statics - a.statics : a.ent.catchAll ? -1 : 1));
}
function goCandidateTiers(url) {
  const tiers = [];
  if (url.startsWith("/v1/")) tiers.push([url]);
  const adminStores = /^\/api\/stores\/([^/]+)(?:\/(.*))?$/.exec(url);
  if (adminStores) tiers.push([`/v1/admin/stores/${adminStores[1]}${adminStores[2] ? "/" + adminStores[2] : ""}`]);
  if (url.startsWith("/api/buyer/")) tiers.push([url.replace(/^\/api\/buyer\//, "/v1/buyer/")]);
  const ents = bffEntriesFor(url);
  const specific = ents.filter((h) => !h.ent.catchAll);
  if (specific.length) tiers.push([...new Set(specific.flatMap((h) => h.ent.lits))]);
  const catchAll = ents.filter((h) => h.ent.catchAll);
  if (catchAll.length) tiers.push([...new Set(catchAll.flatMap((h) => h.ent.lits))]);
  return tiers.filter((t) => t.length);
}
function matchRoutes(goPath) {
  const urlPrefix = goPath.endsWith("*");
  const hits = [];
  for (const r of allRoutes) {
    if (patternMatches(r.pattern, goPath.replace(/\*$/, "{*}"), urlPrefix)) hits.push(r);
  }
  return hits;
}
// route-prefix fallback: a base URL ("/v1/identity", "/api/buyer") that is a strict prefix of registrations still
// proves which HTTP layer package the mode drives, even when no single route is identifiable.
function prefixRegPkgs(goPath) {
  const pkgs = new Set();
  for (const r of allRoutes) {
    const rp = segSplit(r.pattern), us = segSplit(goPath);
    if (us.length >= rp.length) continue;
    let ok = true;
    for (let i = 0; i < us.length; i++) if (!segEq(rp[i], us[i]) && rp[i] !== us[i] && !(rp[i].startsWith("{") && us[i] === "{*}")) { ok = false; break; }
    if (ok) pkgs.add(r.regPkg);
  }
  return [...pkgs];
}

// ---------- per-mode derivation ----------
const entries = modeEntries(read("scripts/dev/test-local.sh"));
const universe = new Set(browserModes(read("scripts/dev/test-local.sh")));
const derivation = {};
const diagnostics = [];
const pkgModeCount = new Map();

for (const e of entries) {
  if (!universe.has(e.name)) continue;
  const body = `${e.prepare}\n${e.run}`;
  const evidence = [];
  const covers = new Set();
  const addCover = (pkg, ev) => {
    if (!isPkg(pkg)) { if (ev) diagnostics.push(`${e.name}: attributed package ${pkg} is not a tracked internal dir (skipped)`); return; }
    covers.add(pkg);
    if (ev) evidence.push(ev);
  };

  // --- OWN Go files ---
  const testNames = [...new Set([...body.matchAll(/\bTest[A-Za-z0-9_]+/g)].map((m) => m[0]))];
  const own = new Set();
  for (const f of tfFiles) {
    const t = tf.get(f);
    if (testNames.some((name) => new RegExp(`^func ${name}[A-Za-z0-9_]*\\(`, "m").test(t.bare))) own.add(f);
  }
  for (const m of body.matchAll(/tests\/foundation\/([A-Za-z0-9_.-]+\.go)/g)) if (tf.has(m[1])) own.add(m[1]);
  for (const f of tfFiles) {
    const t = tf.get(f);
    if (own.has(f)) continue;
    if (testNames.some((name) => t.header.includes(name)) || t.header.includes(e.name)) own.add(f);
  }

  // No Go harness file of its own => node-only mode: it never boots the Go API (its specs run against the Next.js
  // app's own mocks/static data), so no backend path can alter what it exercises — lc_covers stays [] by definition.
  // (lc_fixture is NOT the criterion: --browser-tracking-backfill declares lc_fixture=none yet runs a real Go harness
  // with real PG through test-focused.sh — "reuse the focused runner's machine-wide queue".)
  if (!own.size) {
    derivation[e.name] = [];
    derivation[e.name + "#covers"] = [];
    diagnostics.push(`${e.name}: node-only (no Go harness file; lc_fixture=${e.fixture}) — covers=[] by definition`);
    continue;
  }

  // --- spec/runner files ---
  const specFiles = new Set();
  for (const m of body.matchAll(/((?:tests|apps)\/[A-Za-z0-9_.@/-]+\.(?:spec\.ts|mjs|test\.ts|test\.mjs))/g)) if (existsSync(path.join(root, m[1]))) specFiles.add(m[1]);
  for (const f of own) {
    for (const s of tf.get(f).specs) if (existsSync(path.join(root, s))) specFiles.add(s);
    for (const s of tf.get(f).suites) for (const spec of suites[s] ?? []) if (existsSync(path.join(root, spec))) specFiles.add(spec);
  }

  // --- fixture-call evidence: direct internal imports of OWN files ---
  // (cmd/ binaries an OWN file builds/runs are recorded as INFORMATIONAL diagnostics, not covers: the process tests
  // prove the binary boots and serves the driven flows — the flows themselves are the path evidence below — while the
  // binary's other wiring (e.g. internal/tlsask, mounted only by cmd/api) is never behaviorally exercised by any spec.
  // cmd/** changes still select every PG browser mode through the planner's conservative rule.)
  for (const f of own) {
    const t = tf.get(f);
    for (const p of t.imports) {
      const line = lineOf(t.raw, t.raw.indexOf(`"livecommerce/${p}"`));
      addCover(p, { path: `go-fixture:livecommerce/${p}`, handler: `tests/foundation/${f}:${line}`, package: p, via: "go-fixture-import" });
    }
    for (const c of t.cmdRefs) diagnostics.push(`${e.name}: INFO builds/runs ${c} (direct internal imports: ${[...(cmdImports.get(c) ?? new Map()).keys()].sort().join(", ") || "none"})`);
  }

  // --- path evidence: URLs from specs/runners + OWN Go files (+ Go partial-path fragments) ---
  const urls = new Map(); // normalized url -> source file
  for (const s of specFiles) {
    let text; try { text = read(s); } catch { diagnostics.push(`${e.name}: cannot read spec ${s}`); continue; }
    for (const u of extractUrls(text)) if (!urls.has(u)) urls.set(u, s);
  }
  for (const f of own) for (const u of tf.get(f).urls) if (!urls.has(u)) urls.set(u, `tests/foundation/${f}`);

  for (const [norm, srcFile] of urls) {
    if (norm.startsWith("/__test") || norm.includes("/__test/")) {
      // harness-owned test route: the registering Go file's direct imports are fixture evidence (own files first)
      const lit = norm.replace(/^\//, "");
      const owners = [...own].filter((f) => tf.get(f).raw.includes(lit));
      for (const f of owners.length ? owners : tfFiles.filter((f) => tf.get(f).raw.includes(lit))) {
        const t = tf.get(f);
        for (const p of t.imports) {
          const line = lineOf(t.raw, t.raw.indexOf(`"livecommerce/${p}"`));
          addCover(p, { path: norm, handler: `tests/foundation/${f}:${line}`, package: p, via: owners.length ? "test-route-fixture" : "test-route-fixture-helper" });
        }
      }
      continue;
    }
    let matched = 0;
    for (const tier of goCandidateTiers(norm)) {
      for (const c of tier) for (const r of matchRoutes(c)) {
        matched++;
        const via = r.regPkg === "internal/buyerhttp" ? `buyer-dispatch ${r.kind} @ ${r.dispatch}` : "route-service-call";
        addCover(r.regPkg, { path: norm, handler: `${r.file}:${r.line}`, package: r.regPkg, via: r.regPkg === "internal/buyerhttp" ? `buyer-dispatch ${r.kind} @ ${r.dispatch}` : "route-registration" });
        for (const p of r.pkgs) addCover(p, { path: norm, handler: `${r.file}:${r.line}`, package: p, via });
      }
      if (matched) break;
    }
    if (!matched) {
      const pkgs = prefixRegPkgs(norm.startsWith("/api/buyer") ? norm.replace(/^\/api\/buyer.*/, "/v1/buyer") : norm.startsWith("/api/stores") ? "/v1/admin/stores" : norm);
      if (pkgs.length) for (const p of pkgs) addCover(p, { path: norm, handler: "(route base prefix)", package: p, via: "route-prefix" });
      else diagnostics.push(`${e.name}: UNMAPPED ${norm} (from ${srcFile})`);
    }
  }
  // Go-side partial path literals (strings.Contains(r.URL.Path, "/shipments/tracking-import/")): contiguous-subsequence
  // match against real registrations, >=2 segments with >=1 literal segment (false-positive resistant).
  for (const f of own) for (const frag of tf.get(f).fragments) {
    for (const r of allRoutes) {
      if (!fragmentMatches(r.pattern, frag)) continue;
      const via = r.regPkg === "internal/buyerhttp" ? `buyer-dispatch ${r.kind} @ ${r.dispatch}` : "route-service-call(go-fragment)";
      addCover(r.regPkg, { path: `go-fragment:${frag}`, handler: `${r.file}:${r.line}`, package: r.regPkg, via });
      for (const p of r.pkgs) addCover(p, { path: `go-fragment:${frag}`, handler: `${r.file}:${r.line}`, package: p, via });
    }
  }

  derivation[e.name] = evidence;
  diagnostics.push(`${e.name}: own=${own.size} specs=${specFiles.size} urls=${urls.size} covers=${covers.size}`);
  for (const p of covers) pkgModeCount.set(p, (pkgModeCount.get(p) ?? 0) + 1);
  derivation[e.name + "#covers"] = [...covers].sort();
}

// ---------- outputs ----------
const coversOnly = {};
for (const e of entries) if (universe.has(e.name)) coversOnly[e.name] = derivation[e.name + "#covers"] ?? [];
const shared = Object.keys(SHARED_BACKEND_PACKAGES);
const backendOnly = Object.keys(BACKEND_ONLY_PACKAGES);
const under = (d, p) => d === p || d.startsWith(p + "/");

// ---------- consumer-import rule (leaf join) ----------
// A package no mode names directly but that production code of COVERED packages imports (internal/mail <- identity,
// internal/csvguard <- every export writer, internal/twcity <- customers/migrationimport ...) is exercised by exactly
// the modes that cover those importers: a break in the leaf breaks the importer's flow. One level, production files
// only (no _test.go, no tests/**, cmd/** importers contribute no modes), recorded as consumer-import evidence with the
// importer file:line. Processed shallow-first so a joined parent classifies its own subpackages (mail -> mail/mailtest).
const prodImports = new Map(); // pkg -> [{file, line}] exact imports from tracked production Go files
for (const f of execFileSync("git", ["ls-files", "-z", "--", "*.go"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 }).split("\0").filter(Boolean)) {
  if (f.endsWith("_test.go") || f.startsWith("tests/")) continue;
  const raw = read(f);
  for (const m of raw.matchAll(internalImportRe)) {
    if (!prodImports.has(m[1])) prodImports.set(m[1], []);
    prodImports.get(m[1]).push({ file: f, line: lineOf(raw, m.index) });
  }
}
const zero = pkgDirs.filter((d) => (pkgModeCount.get(d) ?? 0) === 0)
  .sort((a, b) => a.split("/").length - b.split("/").length || (a < b ? -1 : 1));
for (const d of zero) {
  if (shared.some((s) => under(d, s))) continue;       // planner selects all PG modes for it anyway (and its subpackages)
  if (backendOnly.some((b) => under(d, b))) continue;   // explicitly classified: no browser surface
  if (Object.values(coversOnly).some((list) => list.some((c) => under(d, c)))) continue; // covered via a covers ancestor
  const sites = (prodImports.get(d) ?? []).filter((s) => !under(s.file, d) && !s.file.startsWith("cmd/"));
  const targets = new Map(); // mode -> first importer site
  for (const s of sites) {
    const y = path.posix.dirname(s.file);
    for (const [m, list] of Object.entries(coversOnly)) {
      if (targets.has(m)) continue;
      if (list.some((c) => under(y, c))) targets.set(m, s);
    }
  }
  if (!targets.size) {
    diagnostics.push(`REQUIRES-CLASSIFICATION ${d}: no covered production importer (importers: ${(prodImports.get(d) ?? []).map((s) => s.file).join(", ") || "none"}) — list it in BACKEND_ONLY_PACKAGES with a reason`);
    continue;
  }
  for (const [m, s] of targets) {
    coversOnly[m] = [...coversOnly[m], d].sort();
    derivation[m + "#covers"] = coversOnly[m];
    derivation[m].push({ path: `consumer-import:livecommerce/${d}`, handler: `${s.file}:${s.line}`, package: d, via: "consumer-import" });
    pkgModeCount.set(d, (pkgModeCount.get(d) ?? 0) + 1);
  }
  diagnostics.push(`${d}: consumer-import join into ${targets.size} mode(s) via ${(prodImports.get(d) ?? []).filter((s) => !under(s.file, d) && !s.file.startsWith("cmd/")).map((s) => path.posix.dirname(s.file)).filter((v, i, a) => a.indexOf(v) === i).join(", ")}`);
}

const narrow = {};
for (const [m, list] of Object.entries(coversOnly)) narrow[m] = list.filter((p) => !FILE_CLASSIFIED_PACKAGES.includes(p) && !shared.some((s) => under(p, s) || under(s, p)));

// Round 3: preserve ALL round-2 domain-package coverage. Route-file coverage is a conservative join to the
// actual domain services a transport imports (not NewHandler's full API import closure). Broad ${path} URL
// rows in the old derivation are NOT proof of a file-level call: they matched unrelated routes and missed
// live-console's concatenated A2 URL. A direct service-import join also covers those dynamic callers without
// guessing a per-mode allowlist; its ceiling is the existing domain set, with nightly as the safety net.
const fileModes = new Map();
const fileEvidence = {};
const sharedFile = (file) => shared.some((s) => backendPathMatches(file, s));
for (const file of httpapiIdx.files) {
  if (sharedFile(file.rel)) continue;
  const services = [...new Set(file.imports.values())].filter((p) => !shared.some((s) => backendPathMatches(p, s)) && !backendOnly.some((b) => backendPathMatches(p, b)));
  const targets = new Set();
  fileEvidence[file.rel] = { services: services.map((service) => ({ service, site: `${file.rel}:${lineOf(file.raw, file.raw.indexOf(`"livecommerce/${service}"`))}` })), callers: [], basis: "conservative service-domain join, not a direct URL claim" };
  for (const [mode, list] of Object.entries(narrow)) {
    for (const service of services) {
      if (!list.some((p) => backendPathMatches(service, p))) continue;
      targets.add(mode);
    }
  }
  if (!targets.size) throw new Error(`${file.rel}: no domain service coverage; classify explicitly, never infer BACKEND_ONLY from missing URL evidence`);
  fileModes.set(file.rel, targets);
}

// Same-package helper owners inherit all caller domains (e.g. adsRoute, customerRoute, metaConnectScope).
// Only production files participate; shared helper owners already select all Go-booting modes.
let changed;
do {
  changed = false;
  for (const owner of httpapiIdx.files) {
    const targets = fileModes.get(owner.rel);
    if (!targets) continue;
    for (const caller of httpapiIdx.files) {
      if (caller.rel === owner.rel || !fileModes.has(caller.rel)) continue;
      // Bare function values (noPrepare passed by collections.go), not only calls, are dependencies too.
      if (!owner.funcs.some((fn) => new RegExp(`(?<![.\\w])${fn.name}\\b`).test(caller.bare))) continue;
      if (!fileEvidence[owner.rel].callers.includes(caller.rel)) fileEvidence[owner.rel].callers.push(caller.rel);
      for (const mode of fileModes.get(caller.rel)) if (!targets.has(mode)) {
        targets.add(mode); changed = true;
      }
    }
  }
} while (changed);

const allHttpFiles = execFileSync("git", ["ls-files", "-z", "--", "internal/httpapi/"], { cwd: root, encoding: "utf8" }).split("\0").filter((f) => f.endsWith(".go"));
const testOwners = { "internal/httpapi/for_buyer_test.go": "internal/httpapi/live_console.go", "internal/httpapi/refunds_env_test.go": "internal/httpapi/refunds.go" };
for (const file of allHttpFiles) {
  if (sharedFile(file) || !file.endsWith("_test.go")) continue;
  const owner = testOwners[file] ?? file.replace(/_test\.go$/, ".go");
  if (!fileModes.has(owner)) throw new Error(`${file}: test has no classified production sibling; add an evidenced owner or SHARED reason`);
  fileModes.set(file, new Set(fileModes.get(owner)));
  fileEvidence[file] = { test_owner: owner, basis: "production adapter test ownership" };
}
for (const [file, targets] of fileModes) {
  for (const mode of targets) narrow[mode].push(file);
  fileEvidence[file].modes = [...targets].sort();
  diagnostics.push(`FILE ${file}: ${[...targets].sort().join(" ")}`);
}
for (const list of Object.values(narrow)) list.sort();
writeFileSync(path.join(import.meta.dirname, "file-coverage.json"), JSON.stringify(fileEvidence, null, 1) + "\n");

const out = {};
for (const e of entries) if (universe.has(e.name)) out[e.name] = derivation[e.name];
writeFileSync(path.join(import.meta.dirname, "../covers-derivation.json"), JSON.stringify(out, null, 1) + "\n");
writeFileSync(path.join(import.meta.dirname, "covers-narrow.json"), JSON.stringify(narrow, null, 1) + "\n");

// unclassified = what scripts/dev/check-backend-coverage.mjs would red: no covers ancestor, no SHARED ancestor, no BACKEND_ONLY entry
const uncovered = pkgDirs.filter((d) =>
  !FILE_CLASSIFIED_PACKAGES.includes(d) &&
  !Object.values(coversOnly).some((list) => list.some((c) => under(d, c))) &&
  !shared.some((s) => under(d, s)) && !backendOnly.some((b) => under(d, b)));
const importers = new Map();
for (const f of execFileSync("git", ["ls-files", "-z", "--", "*.go"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 }).split("\0").filter(Boolean)) {
  const raw = read(f);
  for (const m of raw.matchAll(internalImportRe)) {
    const hit = uncovered.filter((u) => m[1] === u || m[1].startsWith(u + "/")).sort((a, b) => b.length - a.length)[0];
    if (hit) { if (!importers.has(hit)) importers.set(hit, new Set()); importers.get(hit).add(f); }
  }
}
const report = [
  ...diagnostics,
  "",
  `SHARED_BACKEND_PACKAGES (subtracted from covers-narrow): ${shared.join(", ")}`,
  `route table: httpapi/identityhttp HandleFunc=${handleRoutes.length}, buyerhttp kinds=${buyerRoutes.length}; route diag: ${routeDiag.length}`,
  ...routeDiag.map((d) => `  ROUTE ${d}`),
  "",
  `per-package mode counts (raw evidence, ${pkgDirs.length} internal dirs):`,
  ...pkgDirs.map((d) => `  ${d}: ${pkgModeCount.get(d) ?? 0}`).sort((a, b) => parseInt(b.split(": ")[1]) - parseInt(a.split(": ")[1])),
  "",
  `uncovered by any browser mode evidence: ${uncovered.length}`,
  ...uncovered.map((u) => `  ${u} <- imported by: ${[...(importers.get(u) ?? [])].sort().join(", ") || "(no Go importer)"}`),
  "",
  ...Object.entries(coversOnly).map(([m, l]) => `${m}: ${l.length} pkgs -> ${l.join(" ")}`),
].join("\n");
writeFileSync(path.join(import.meta.dirname, "r2-diagnostics.txt"), report);
console.log(`derive-narrow-covers: ${Object.keys(out).length} modes; uncovered=${uncovered.length}; see r2-diagnostics.txt`);
