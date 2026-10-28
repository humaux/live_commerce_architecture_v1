#!/usr/bin/env node
// Purpose: derive PR browser execution ownership from the native registry, Go harnesses and spec imports.
// Depends on: pr-modes.mjs registry parser, tracked tests, playwright.config.ts and registry server/build declarations.
// Used by: pr-modes.mjs; backend-coverage.test.mjs checks the committed artifact for drift in CI.
// Invariants: no mode hand-list; deterministic static analysis only; unresolved dependencies never justify narrowing.
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { modeEntries, browserModes } from "./pr-modes.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const args = process.argv.slice(2);
const refIndex = args.indexOf("--source-ref");
const sourceRef = refIndex < 0 ? null : execFileSync("git", ["rev-parse", "--verify", `${args[refIndex + 1]}^{commit}`], { cwd: root, encoding: "utf8" }).trim();
// Read-only source revisions let a worker prepare merge-checkout data without merging or modifying shared branches.
const read = (file) => sourceRef
  ? execFileSync("git", ["show", `${sourceRef}:${file}`], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 })
  : readFileSync(path.join(root, file), "utf8");
const hash = (text) => createHash("sha256").update(text).digest("hex");
const inventory = execFileSync("git", sourceRef ? ["ls-tree", "-r", "--name-only", "-z", sourceRef] : ["ls-files", "-z"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 }).split("\0").filter(Boolean).sort();
const tracked = new Set(inventory);
const registry = read("scripts/dev/test-local.sh");
const config = read("playwright.config.ts");
const suites = {};
const suiteBlock = /const suites[^=]*=\s*\{([\s\S]*?)\n\s*\};/.exec(config)?.[1];
if (!suiteBlock) throw new Error("cannot derive Playwright suite map");
for (const m of suiteBlock.matchAll(/(?:"([^"]+)"|'([^']+)'|([A-Za-z_$][\w$]*))\s*:\s*\[([^\]]*)\]/g))
  suites[m[1] ?? m[2] ?? m[3]] = [...m[4].matchAll(/"([^"]+\.(?:spec|test)\.ts)"/g)].map((s) => `tests/admin/${s[1]}`);
const goFiles = inventory.filter((file) => /^tests\/foundation\/.*\.go$/.test(file));
const go = new Map(goFiles.map((file) => [file, read(file)]));
// Discover suite-parameter forwarders from Go code, so helper names are evidence rather than a new hand list.
const forwarders = new Set();
const goFunctions = [...go.values()].flatMap((raw) => [...raw.matchAll(/^func (?:\([^\n]+\) )?([A-Za-z0-9_]+)\([^]*?(?=^func |$(?![^]))/gm)].map((m) => ({ name: m[1], body: m[0] })));
let more = true;
while (more) {
  more = false;
  for (const fn of goFunctions) if (!forwarders.has(fn.name) &&
    (fn.body.includes('"LC_BROWSER_SUITE"') || [...forwarders].some((name) => new RegExp(`\\b${name}\\(`).test(fn.body)))) {
    forwarders.add(fn.name); more = true;
  }
}
const sources = {};
// Hash discovery inputs, not app implementations: component edits need no regeneration; a new spec/import/server does.
for (const file of inventory.filter((file) => /^tests\/(?:(?:admin|e2e|ui|storefront|helpers|foundation)\/.*|browser-evidence)\.(?:go|[cm]?[jt]sx?|json)$/.test(file))) sources[file] = hash(read(file));
sources["scripts/dev/test-local.sh"] = hash(registry);
sources["playwright.config.ts"] = hash(config);
const sorted = (set) => [...set].sort();
const specDirs = ["tests/admin", "tests/storefront", "tests/e2e", "tests/ui", "apps/admin/tests", "apps/storefront/tests"];
// Small lexical scanner, not regex over raw source: fixture strings and comments often contain fake imports.
// Template bodies are opaque; a computed module specifier is returned as null and makes its owner conservative.
function moduleReferences(raw) {
  const tokens = [];
  for (let i = 0; i < raw.length;) {
    const c = raw[i];
    if (/\s/.test(c)) { i++; continue; }
    if (raw.startsWith("//", i)) { const end = raw.indexOf("\n", i); i = end < 0 ? raw.length : end; continue; }
    if (raw.startsWith("/*", i)) { const end = raw.indexOf("*/", i + 2); i = end < 0 ? raw.length : end + 2; continue; }
    // Regex literals can contain quote/import text too; skip them as one opaque token.
    if (c === "/" && (!tokens.length || ["(", "=", ",", ":", "[", "!", "return", "=>"].includes(tokens.at(-1).value))) {
      let j = i + 1, bracket = false;
      for (; j < raw.length; j++) {
        if (raw[j] === "\\") { j++; continue; }
        if (raw[j] === "[") bracket = true;
        if (raw[j] === "]") bracket = false;
        if (raw[j] === "/" && !bracket) break;
        if (raw[j] === "\n") break;
      }
      if (raw[j] === "/") { i = j + 1; while (/[a-z]/i.test(raw[i] ?? "") && i < raw.length) i++; tokens.push({ value: "regex" }); continue; }
    }
    if (c === '"' || c === "'" || c === "`") {
      const quote = c; let value = ""; i++;
      while (i < raw.length && raw[i] !== quote) {
        if (raw[i] === "\\") { value += raw[i++] + (raw[i++] ?? ""); } else value += raw[i++];
      }
      i++; tokens.push({ value, string: true, computed: quote === "`" && value.includes("${") }); continue;
    }
    const word = /^[A-Za-z_$][\w$]*/.exec(raw.slice(i))?.[0];
    tokens.push({ value: word ?? c, string: false }); i += word?.length ?? 1;
  }
  const refs = [];
  for (let i = 0; i < tokens.length; i++) {
    const t = tokens[i], next = tokens[i + 1];
    if (t.string || !next) continue;
    if (t.value === "from" && next.string) refs.push(next.value);
    if (t.value === "import" && next.string) refs.push(next.value);
    if ((t.value === "import" || t.value === "require") && next.value === "(") {
      const arg = tokens[i + 2];
      refs.push(arg?.string && !arg.computed ? arg.value : null);
    }
  }
  return refs;
}
// Only installed/workspace package names are external. Unknown aliases cannot hide a local helper dependency.
const externals = new Set();
for (const file of inventory.filter((file) => file === "package.json" || /^(?:apps|packages)\/[^/]+\/package\.json$/.test(file))) {
  const manifest = JSON.parse(read(file));
  for (const name of [...Object.keys(manifest.dependencies ?? {}), ...Object.keys(manifest.devDependencies ?? {}), ...Object.keys(manifest.peerDependencies ?? {}), manifest.name].filter(Boolean)) externals.add(name);
  sources[file] = hash(read(file));
}
// Foundation's existing Node suite is authoritative for prep tests that never drive a browser.
const foundationSource = read("scripts/dev/test-node.sh");
const foundationRoots = new Set([...foundationSource.matchAll(/tests\/[A-Za-z0-9_./-]+\.test\.[cm]?[jt]s/g)].map((m) => m[0]));
sources["scripts/dev/test-node.sh"] = hash(foundationSource);
const modes = {};
for (const entry of modeEntries(registry).filter((entry) => browserModes(registry).includes(entry.name))) {
  const specs = new Set(), inputs = new Set(), apps = new Set(), unresolved = new Set();
  const evidence = new Set([`scripts/dev/test-local.sh:${entry.name}:lc_build=${entry.build}`]);
  const body = `${entry.prepare}\n${entry.run}`;
  const runPatterns = [...body.matchAll(/-run(?:=|\s+)["']([^"']+)/g)].map((m) => new RegExp(m[1]));
  const tests = [...new Set([...body.matchAll(/\bTest[A-Za-z0-9_]+(?![A-Za-z0-9_(])/g)].map((m) => m[0]))];
  const selectedTests = new Set(tests);
  for (const raw of go.values()) for (const m of raw.matchAll(/^func (Test[A-Za-z0-9_]+)\(/gm))
    if (runPatterns.some((pattern) => pattern.test(m[1]))) selectedTests.add(m[1]);
  // Resolve actual -run regexes before OWN-file attribution: TestBrowser(A|B) is not the prefix TestBrowser*.
  const own = goFiles.filter((file) => {
    const raw = go.get(file), header = raw.split("\n").slice(0, 16).join("\n");
    return [...selectedTests].some((name) => new RegExp(`^func ${name}[A-Za-z0-9_]*\\(`, "m").test(raw) || header.includes(name)) ||
      header.includes(entry.name) || body.includes(file);
  });
  function addSpec(file, from) {
    if (!tracked.has(file)) return; // Source literals also include diagnostic/sample output, not executable spec arguments.
    specs.add(file); evidence.add(`${from} -> ${file}`);
  }
  function scan(raw, from) {
    for (const m of raw.matchAll(/((?:tests|apps)\/[A-Za-z0-9_.@/-]+\.(?:spec\.[jt]s|[cm]js|test\.[cm]?[jt]s))/g)) addSpec(m[1], from);
    for (const m of raw.matchAll(/["']([A-Za-z0-9_.@-]+\.(?:spec\.[jt]s|[cm]js))["']/g)) {
      const matches = specDirs.map((dir) => `${dir}/${m[1]}`).filter((file) => tracked.has(file));
      for (const file of matches) addSpec(file, from);
    }
    const suiteTexts = [...raw.matchAll(/LC_BROWSER_SUITE[\\"'\s]*(?::|=)[\\"'\s]*([a-z0-9-]+)/g)].map((m) => m[1]);
    for (const line of raw.split("\n")) if ([...forwarders].some((name) => new RegExp(`\\b${name}\\(`).test(line)))
      for (const m of line.matchAll(/["']([a-z0-9-]+)["']/g)) suiteTexts.push(m[1]);
    for (const suite of suiteTexts) for (const file of suites[suite] ?? []) addSpec(file, `${from}:suite=${suite}`);
    for (const m of raw.matchAll(/apps\/([a-z0-9-]+)(?:\/|["'])/g)) {
      apps.add(m[1]); evidence.add(`${from}:server/import apps/${m[1]}`);
    }
    for (const m of raw.matchAll(/--filter\s+["']?(?:@live-commerce\/)?([a-z0-9-]+)/g)) {
      if (inventory.some((file) => file.startsWith(`apps/${m[1]}/`))) {
        apps.add(m[1]); evidence.add(`${from}:build apps/${m[1]}`);
      }
    }
  }
  if (entry.build === "both") { apps.add("admin"); apps.add("storefront"); }
  else if (entry.build !== "none") apps.add(entry.build);
  scan(body, `registry:${entry.name}`);
  for (const file of own) { evidence.add(`harness:${file}`); scan(go.get(file), file); }

  // Exact relative-import closure, including re-exports, require(), literal import(), JSON fixtures and referenced files.
  // External packages do not narrow tests: packages/** and lock/config changes always select the universe.
  const browserInputs = new Set(), foundationInputs = new Set(), visited = new Set();
  const queue = [...specs].map((file) => ({ file, foundation: foundationRoots.has(file) }));
  while (queue.length) {
    const { file, foundation } = queue.shift();
    const visit = `${foundation}:${file}`;
    if (visited.has(visit)) continue;
    visited.add(visit);
    inputs.add(file);
    (foundation ? foundationInputs : browserInputs).add(file);
    const raw = read(file);
    if (!/\.[cm]?[jt]sx?$/.test(file)) continue;
    for (const m of raw.matchAll(/apps\/([a-z0-9-]+)(?:\/|["'])/g)) {
      apps.add(m[1]); evidence.add(`${file}:server/import apps/${m[1]}`);
    }
    const references = moduleReferences(raw);
    for (const ref of references) {
      if (ref === null) {
        unresolved.add(`${file}: computed module import`);
        continue;
      }
      if (!ref.startsWith(".")) {
        const pkg = ref.startsWith("@") ? ref.split("/").slice(0, 2).join("/") : ref.split("/")[0];
        if (!ref.startsWith("node:") && !externals.has(pkg)) unresolved.add(`${file}: unresolved import ${ref}`);
        continue;
      }
      const base = path.posix.normalize(path.posix.join(path.posix.dirname(file), ref));
      const candidates = [base, ...[".ts", ".tsx", ".mjs", ".js", ".cjs", ".json", "/index.ts", "/index.tsx", "/index.js"].map((ext) => base + ext)];
      // TS commonly writes .js imports which resolve to .ts source.
      if (base.endsWith(".js")) candidates.push(base.slice(0, -3) + ".ts", base.slice(0, -3) + ".tsx");
      const target = candidates.find((candidate) => tracked.has(candidate));
      if (!target) {
        if (!base.includes("/node_modules/")) unresolved.add(`${file}: unresolved import ${ref}`);
        continue;
      }
      const app = /^apps\/([^/]+)\//.exec(target)?.[1];
      if (app) { apps.add(app); evidence.add(`${file}:import ${target}`); }
      if (target.startsWith("tests/")) queue.push({ file: target, foundation });
    }
    // Non-module fixture files explicitly read by a driver (e.g. a baseline JSON) are execution inputs too.
    for (const m of raw.matchAll(/(?:readFileSync|readFile|new URL)\(\s*["'](tests\/(?:admin|e2e|ui|storefront)\/[^"']+)["']/g))
      if (tracked.has(m[1])) queue.push({ file: m[1], foundation });
  }
  const browserUnresolved = sorted(unresolved).filter((issue) => browserInputs.has(issue.split(":")[0]));
  const foundationUnresolvedInputs = sorted(new Set(sorted(unresolved).map((issue) => issue.split(":")[0]).filter((file) => !browserInputs.has(file))));
  for (const file of foundationUnresolvedInputs) evidence.add(`scripts/dev/test-node.sh:foundation covers computed Node loader ${file}`);
  modes[entry.name] = { specs: sorted(specs), inputs: sorted(inputs), apps: sorted(apps), unresolved: browserUnresolved,
    foundationInputs: sorted(foundationInputs), foundationUnresolvedInputs, evidence: sorted(evidence) };
}
const artifact = JSON.stringify({ version: 1, registryHash: hash(registry), sources, modes }, null, 2) + "\n";
const outputIndex = args.indexOf("--output");
const output = path.resolve(root, outputIndex < 0 ? "output/ci-select-backend-browser/browser-ownership.json" : args[outputIndex + 1]);
if (args.includes("--check")) {
  let current;
  try { current = readFileSync(output, "utf8"); } catch { /* Missing artifact is drift. */ }
  if (current !== artifact) { console.error("browser ownership drift: regenerate with node scripts/dev/derive-browser-ownership.mjs"); process.exitCode = 1; }
} else {
  mkdirSync(path.dirname(output), { recursive: true });
  writeFileSync(output, artifact);
  console.log(`browser ownership: wrote ${output} (${Object.keys(modes).length} modes)`);
}
