// G-UI3/G-UI5 ratchet: legacy allowances can only shrink; new shell files have no waivers.
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import { createRequire } from "node:module";
import { createHash } from "node:crypto";
// TS 7 keeps the build CLI; this dev-only alias supplies the in-process compiler API.
const ts = createRequire(import.meta.url)("typescript-api");
const compilerOptions = {
  moduleResolution: ts.ModuleResolutionKind.Bundler,
  baseUrl: resolve("apps/admin"),
  paths: { "@/*": ["./*"] },
};
export function imports(file, source) {
  const result = [];
  const tree = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true);
  function visit(node) {
    const name =
      ts.isImportDeclaration(node) || ts.isExportDeclaration(node)
        ? node.moduleSpecifier
        : ts.isCallExpression(node) &&
            node.expression.kind === ts.SyntaxKind.ImportKeyword
          ? node.arguments[0]
          : undefined;
    if (name && ts.isStringLiteral(name)) {
      const target = ts.resolveModuleName(
        name.text,
        resolve(file),
        compilerOptions,
        ts.sys,
      ).resolvedModule?.resolvedFileName;
      if (target) result.push(resolve(target));
    }
    ts.forEachChild(node, visit);
  }
  visit(tree);
  return result;
}
export function deepFeatureImport(file, target) {
  const domain = file.match(/\/features\/([^/]+)/)?.[1];
  const other = target.match(/\/features\/([^/]+)/)?.[1];
  return Boolean(
    domain && other && domain !== other && !/\/index\.[tm]sx?$/.test(target),
  );
}
export const walk = (dir) =>
  existsSync(dir)
    ? readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
        e.isDirectory()
          ? ["node_modules", ".next"].includes(e.name)
            ? []
            : walk(dir + "/" + e.name)
          : [dir + "/" + e.name],
      )
    : [];
export function findings(path, source) {
  const clean = source
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .split("\n")
    .filter((line) => !/^\s*\/\//.test(line))
    .join("\n");
  const out = {};
  let minorCopy = 0;
  const tree = ts.createSourceFile(path, source, ts.ScriptTarget.Latest, true);
  function visit(node) {
    if (
      (ts.isStringLiteralLike(node) || ts.isJsxText(node)) &&
      /最小货币单位|最小貨幣單位|\bminor units?\b|smallest currency/i.test(
        node.text,
      )
    )
      minorCopy++;
    ts.forEachChild(node, visit);
  }
  visit(tree);
  if (minorCopy) out.minorCopy = minorCopy;
  if (!path.startsWith("packages/format/")) {
    const count = (
      clean.match(
        /\bIntl\.(?:NumberFormat|DateTimeFormat)\s*\(|\.toLocale(?:String|DateString|TimeString)\s*\(/g,
      ) || []
    ).length;
    if (count) out.format = count;
  }
  if (
    !/(?:\/api\.ts|\/api\/.*|[-/]client\.[tm]sx?|\/lib\/(?:auth|backend)\.ts)$/.test(
      path,
    )
  ) {
    const count = (clean.match(/\bfetch\s*\(/g) || []).length;
    if (count) out.fetch = count;
  }
  const lines = source.split("\n").length;
  if (lines > 800) out.lines = lines;
  return out;
}
export function validateAllowances(allowed, basis) {
  const errors = [];
  for (const [file, limits] of Object.entries(allowed))
    for (const [rule, limit] of Object.entries(limits))
      if (
        !basis[file] ||
        !basis[file][rule] ||
        limit > basis[file][rule] ||
        !Number.isInteger(limit) ||
        limit < 1
      )
        errors.push("allowlist expanded: " + file + ":" + rule);
  return errors;
}
export function architectureGate() {
  const basisText = readFileSync("tests/admin/ui-legacy-basis.json", "utf8");
  if (
    createHash("sha256").update(basisText).digest("hex") !==
    "f6b85e6263d2c8aa2844d56f1b6c4cc1ef64364c2a08a537fcf3b58bef12dbc2"
  )
    return {
      errors: ["W0 immutable b4223c8 legacy ceiling was changed"],
      warnings: [],
    };
  const basis = JSON.parse(basisText);
  const allowed = JSON.parse(
    readFileSync("tests/admin/ui-legacy-allowlist.json", "utf8"),
  );
  const errors = validateAllowances(allowed, basis),
    warnings = [];
  const sources = Object.fromEntries(
    ["apps/admin", "apps/storefront", "packages/ui", "packages/format"]
      .flatMap(walk)
      .filter(
        (p) =>
          /\.[tm]sx?$/.test(p) &&
          !p.includes("/tests/") &&
          !p.endsWith(".d.ts"),
      )
      .map((p) => [p, readFileSync(p, "utf8")]),
  );
  for (const [file, source] of Object.entries(sources)) {
    for (const [rule, count] of Object.entries(findings(file, source)))
      if (count > (allowed[file]?.[rule] ?? 0))
        errors.push(
          file +
            ": " +
            rule +
            " " +
            count +
            " > " +
            (allowed[file]?.[rule] ?? 0),
        );
    if (source.split("\n").length > 500 && source.split("\n").length <= 800)
      warnings.push(file + ": >500 lines");
    if (file.startsWith("apps/admin/src/") || file.startsWith("packages/")) {
      for (const target of imports(file, source)) {
        if (deepFeatureImport(file, target))
          errors.push(file + ": deep cross-feature import " + target);
      }
    }
  }
  // New-module dependency graph only. Legacy modules are terminal nodes, not silently rewritten.
  const graph = new Map();
  for (const [file, source] of Object.entries(sources).filter(
    ([p]) => p.startsWith("apps/admin/src/") || p.startsWith("packages/"),
  )) {
    graph.set(resolve(file), imports(file, source));
  }
  const done = new Set(),
    active = new Set();
  function visit(node) {
    if (active.has(node)) {
      errors.push("import cycle: " + node);
      return;
    }
    if (done.has(node)) return;
    active.add(node);
    for (const next of graph.get(node) || []) visit(next);
    active.delete(node);
    done.add(node);
  }
  for (const node of graph.keys()) visit(node);
  return { errors, warnings };
}
if (process.argv[1]?.endsWith("/ui-architecture-gate.mjs")) {
  const { errors, warnings } = architectureGate();
  for (const w of warnings) console.log("WARN G-UI5 " + w);
  for (const e of errors) console.error("FAIL " + e);
  console.log(errors.length ? "FAIL G-UI3/G-UI5" : "PASS G-UI3/G-UI5");
  if (errors.length) process.exitCode = 1;
}
