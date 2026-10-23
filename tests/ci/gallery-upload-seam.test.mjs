// Purpose: #147 TH-13 keeps the simulated gallery upload hold/release seam out of every admin build except the flag-enabled gallery build.
// Depends on: node:test/assert/fs/path and the pinned TypeScript parser; every non-test TS/JS source file under apps/admin is inspected as data.
// Used by: scripts/dev/test-node.sh (Foundation CI); no browser, build, server or provider is started here.
//
// Reachability claimed by this gate: the checked-in spellings (Identifier, StringLiteral, NoSubstitutionTemplateLiteral, literal element
// access) and any computed global property access or fragment-built template/concatenation in apps/admin production source. Names built
// without any literal fragment (e.g. String.fromCharCode) are UNSUPPORTED and not claimed; they are also not used in this repository.
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript-api";

const root = fileURLToPath(new URL("../../", import.meta.url));
const ADMIN = path.join(root, "apps/admin");
const FLAG = 'process.env.NEXT_PUBLIC_LC_GALLERY_TEST === "1"';
const HOLD = "__lcHoldUpload";
const RELEASE = "lc-gallery-upload-release";
const SOURCE = /\.(?:[cm]?[jt]sx?)$/;
const TEST_FILE = /\.(?:test|spec)\.[cm]?[jt]sx?$/;
const SKIP_DIR = new Set(["node_modules", ".next", ".turbo", "coverage", "test", "tests", "__tests__"]);
const DYNAMIC_FRAGMENT = /(?:__lc|lc-gallery|HoldUpload|upload-release)/;
const GLOBAL_OBJECTS = new Set(["window", "globalThis", "self", "global"]);

function scriptKind(name) {
  if (name.endsWith(".tsx")) return ts.ScriptKind.TSX;
  if (name.endsWith(".jsx")) return ts.ScriptKind.JSX;
  if (/\.(?:js|mjs|cjs)$/.test(name)) return ts.ScriptKind.JS;
  return ts.ScriptKind.TS;
}

/** The global object name behind a (possibly cast/parenthesized) expression, or null. */
function globalName(expression) {
  let node = expression;
  while (node && (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) || ts.isNonNullExpression(node))) node = node.expression;
  return node && ts.isIdentifier(node) && GLOBAL_OBJECTS.has(node.text) ? node.text : null;
}

/** Every non-test source file under apps/admin (excluding generated output and dependencies), as parser input. */
export function adminSources(dir = ADMIN) {
  const sources = [];
  const walk = (current) => {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const full = path.join(current, entry.name);
      if (entry.isDirectory()) { if (!SKIP_DIR.has(entry.name)) walk(full); continue; }
      if (!SOURCE.test(entry.name) || TEST_FILE.test(entry.name)) continue;
      sources.push({ name: path.relative(root, full), text: readFileSync(full, "utf8") });
    }
  };
  walk(dir);
  return sources.sort((a, b) => a.name.localeCompare(b.name));
}

/**
 * Every seam-spelling occurrence must sit inside the enabled branch of the literal public build-time flag. Any other occurrence, any
 * computed global property name and any fragment-built template/concatenation is a violation: this gate never awards a "some occurrence
 * is guarded" pass.
 */
export function scanSeamSources(sources) {
  const violations = [];
  const counts = { hold: 0, release: 0 };
  for (const { name, text } of sources) {
    const file = ts.createSourceFile(name, text, ts.ScriptTarget.Latest, true, scriptKind(name));
    const at = (node) => `${name}:${file.getLineAndCharacterOfPosition(node.getStart(file)).line + 1}`;
    const insideGuard = (node) => {
      for (let parent = node.parent; parent; parent = parent.parent) {
        if (ts.isIfStatement(parent) && parent.expression.getText(file) === FLAG &&
            node.pos >= parent.thenStatement.pos && node.end <= parent.thenStatement.end) return true;
      }
      return false;
    };
    const marker = (node, kind) => {
      counts[kind]++;
      if (!insideGuard(node)) violations.push(`${at(node)} ${kind} is outside the literal public build-time flag branch`);
    };
    function visit(node) {
      if (ts.isIdentifier(node) && node.text === HOLD) marker(node, HOLD);
      else if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
        if (node.text === HOLD) marker(node, HOLD);
        else if (node.text.includes(RELEASE)) marker(node, RELEASE);
      }
      if (ts.isElementAccessExpression(node) && globalName(node.expression) &&
          !ts.isStringLiteral(node.argumentExpression) && !ts.isNoSubstitutionTemplateLiteral(node.argumentExpression)) {
        violations.push(`${at(node)} computed ${globalName(node.expression)}[...] name is forbidden by the seam gate`);
      }
      if ((ts.isTemplateExpression(node) || ts.isBinaryExpression(node)) && DYNAMIC_FRAGMENT.test(node.getText(file))) {
        violations.push(`${at(node)} seam name built from fragments is forbidden by the seam gate`);
      }
      ts.forEachChild(node, visit);
    }
    visit(file);
  }
  return { violations, counts };
}

const seam = 'window.__lcHoldUpload; window.addEventListener("lc-gallery-upload-release", finish);';
const guarded = `if (${FLAG}) { ${seam} }`;

test("every gallery seam marker in apps/admin production source sits inside the literal flag branch", () => {
  const sources = adminSources();
  assert.ok(sources.length > 100, `expected the whole production corpus, scanned ${sources.length} files`);
  assert.ok(sources.some(({ name }) => name.endsWith("UiGalleryTemplates.tsx")), "the gallery component must be in the scanned corpus");
  const { violations, counts } = scanSeamSources(sources);
  assert.deepEqual(violations, []);
  assert.ok(counts.hold > 0, "the hold fixture must remain covered, not deleted to satisfy the guard");
  assert.ok(counts.release > 0, "the release fixture must remain covered, not deleted to satisfy the guard");
});

test("the scanner rejects every ungated spelling, including the shapes the previous visitor missed", () => {
  const fixtures = {
    ungated: seam,
    "runtime-only flag": `if (process.env.LC_UI_GALLERY === "1") { ${seam} }`,
    "element-access flag read": `if (process.env["NEXT_PUBLIC_LC_GALLERY_TEST"] === "1") { ${seam} }`,
    "else branch": `if (${FLAG}) {} else { ${seam} }`,
    "after the branch": `if (${FLAG}) {} ${seam}`,
    "element access": `if (${FLAG}) { window.__lcHoldUpload; } if (window["__lcHoldUpload"]) { window.addEventListener("lc-gallery-upload-release", finish); }`,
    "template literal event": `if (${FLAG}) { window.__lcHoldUpload; window.addEventListener(\`lc-gallery-upload-release\`, finish); }`,
    "template built from fragments": 'const event = `lc-gallery-upload-${suffix}`; window.addEventListener(event, finish);',
    "concatenated name": 'const event = "lc-gallery-" + "upload-release"; window.addEventListener(event, finish);',
    "computed global name": 'const key = ["__lc", "HoldUpload"].join(""); window[key];',
    "alias guard": `const env = process.env; if (env.NEXT_PUBLIC_LC_GALLERY_TEST === "1") { ${seam} }`,
  };
  for (const [name, fixture] of Object.entries(fixtures)) {
    const { violations } = scanSeamSources([{ name: "fixture.tsx", text: `${guarded}\n${fixture}` }]);
    assert.ok(violations.length > 0, `${name} must be rejected`);
  }
});

test("a seam moved into another production module is still found", () => {
  const { violations } = scanSeamSources([
    { name: "apps/admin/components/Gallery.tsx", text: guarded },
    { name: "apps/admin/lib/helper.ts", text: 'export function helper() { window.addEventListener("lc-gallery-upload-release", finish); }' },
  ]);
  assert.equal(violations.length, 1);
  assert.match(violations[0], /apps\/admin\/lib\/helper\.ts:1 lc-gallery-upload-release/);
});

test("markers nested inside the guarded branch stay legal", () => {
  const source = `if (${FLAG}) { useEffect(() => { window.addEventListener("lc-gallery-upload-release", finish); }); ` +
    "if ((window as unknown as { __lcHoldUpload?: boolean }).__lcHoldUpload) {}\n}";
  const { violations, counts } = scanSeamSources([{ name: "ok.tsx", text: source }]);
  assert.deepEqual(violations, []);
  assert.deepEqual(counts, { hold: 1, release: 1 });
});
