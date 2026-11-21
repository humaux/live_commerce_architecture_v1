// Purpose: detect missing members and path-ambiguous responses in browser specs outside the app tsconfigs.
// Depends on: the workspace TypeScript compiler API and Playwright declarations.
// Used by: the browser-spec-types PR job and tests/ci/ci-plan.test.mjs.
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { navigationResponses } from "./check-navigation-response.mjs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript-api";

const options = {
  strict: true, noEmit: true, skipLibCheck: true, allowImportingTsExtensions: true,
  target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext,
  moduleResolution: ts.ModuleResolutionKind.Bundler, jsx: ts.JsxEmit.ReactJSX,
  lib: ["lib.dom.d.ts", "lib.dom.iterable.d.ts", "lib.esnext.d.ts"],
  typeRoots: ["./apps/admin/node_modules/@types"], types: ["node"],
};

export function checkSpecTypes(files) {
  if (!files.length) throw new Error("No tracked browser specs found");
  const program = ts.createProgram(files, options);
  const setup = [...program.getOptionsDiagnostics(), ...program.getGlobalDiagnostics()];
  if (setup.length) throw new Error(setup.map(d => ts.flattenDiagnosticMessageText(d.messageText, "\n")).join("\n"));

  const errors = [];
  for (const file of files) {
    const source = program.getSourceFile(file);
    if (!source) throw new Error(`Cannot read tracked browser spec: ${file}`);
    // An unresolved Playwright installation turns its API into any, masking argument errors.
    if (!ts.resolveModuleName("@playwright/test", file, options, ts.sys).resolvedModule)
      throw new Error(`${file}: install the frozen workspace dependencies to resolve @playwright/test`);
    errors.push(...navigationResponses(source));
    for (const d of program.getSemanticDiagnostics(source)) {
      if (d.code !== 2339) continue;
      const fix = "Use a member on the declared receiver type, or align the fixture type with its actual producer.";
      const { line, character } = source.getLineAndCharacterOfPosition(d.start);
      errors.push(`${file}:${line + 1}:${character + 1} TS${d.code} ${ts.flattenDiagnosticMessageText(d.messageText, "\n")}\n  Fix: ${fix}`);
    }
  }
  return errors;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const files = execFileSync("git", ["ls-files", "-z", "--", "tests/**/*.spec.ts", "tests/**/*.acceptance.ts"], { encoding: "utf8" }).split("\0").filter(Boolean);
  const errors = checkSpecTypes(files);
  const jsSources = execFileSync("git", ["ls-files", "-z", "--", "tests/admin/*.mjs", "tests/storefront/*.mjs", "tests/e2e/*.mjs"], { encoding: "utf8" })
    .split("\0").filter(file => file && !file.endsWith(".test.mjs"));
  for (const file of jsSources) {
    const source = ts.createSourceFile(file, readFileSync(file, "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
    errors.push(...navigationResponses(source));
  }
  for (const error of errors) console.error(error);
  console.log(`Browser spec types: ${files.length} typed specs, ${jsSources.length} JS sources, ${errors.length} findings`);
  process.exitCode = errors.length ? 1 : 0;
}
