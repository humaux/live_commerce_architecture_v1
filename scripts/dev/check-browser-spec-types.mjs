// Browser specs run through Playwright's transpiler outside both application tsconfigs.
import { execFileSync } from "node:child_process";
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

function isPlaywrightCall(checker, source, position) {
  let node = ts.getTokenAtPosition(source, position);
  // The offending argument may itself be a call, e.g. page.getByText(makeNumber()).
  while (node && !(ts.isCallExpression(node) && node.arguments.some(arg =>
    arg.getStart(source) <= position && position < arg.end))) node = node.parent;
  const declaration = node && checker.getResolvedSignature(node)?.declaration;
  return declaration && /\/node_modules\/(?:playwright(?:-core)?|@playwright\/test)\//.test(declaration.getSourceFile().fileName);
}

export function checkSpecTypes(files) {
  if (!files.length) throw new Error("No tracked browser specs found");
  const program = ts.createProgram(files, options);
  const checker = program.getTypeChecker();
  const setup = [...program.getOptionsDiagnostics(), ...program.getGlobalDiagnostics()];
  if (setup.length) throw new Error(setup.map(d => ts.flattenDiagnosticMessageText(d.messageText, "\n")).join("\n"));

  const errors = [];
  for (const file of files) {
    const source = program.getSourceFile(file);
    if (!source) throw new Error(`Cannot read tracked browser spec: ${file}`);
    // An unresolved Playwright installation turns its API into any, masking argument errors.
    if (!ts.resolveModuleName("@playwright/test", file, options, ts.sys).resolvedModule)
      throw new Error(`${file}: install the frozen workspace dependencies to resolve @playwright/test`);
    for (const d of program.getSemanticDiagnostics(source)) {
      let fix;
      if (d.code === 2339) fix = "Use the declared field name, or add the actual producer field to the fixture type.";
      else if ([2345, 2769].includes(d.code) && isPlaywrightCall(checker, source, d.start))
        fix = "Match the Playwright signature shown above. For a widened callback payload, preserve the literal at the call site (e.g. mode: 'INIT' as const).";
      else continue;
      const { line, character } = source.getLineAndCharacterOfPosition(d.start);
      errors.push(`${file}:${line + 1}:${character + 1} TS${d.code} ${ts.flattenDiagnosticMessageText(d.messageText, "\n")}\n  Fix: ${fix}`);
    }
  }
  return errors;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const files = execFileSync("git", ["ls-files", "-z", "--", "tests/**/*.spec.ts", "tests/**/*.acceptance.ts"], { encoding: "utf8" }).split("\0").filter(Boolean);
  const errors = checkSpecTypes(files);
  for (const error of errors) console.error(error);
  console.log(`Browser spec types: ${files.length} files, ${errors.length} findings`);
  process.exitCode = errors.length ? 1 : 0;
}
