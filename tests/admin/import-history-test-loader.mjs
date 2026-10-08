// Purpose: in-memory Node loading for actual admin TS/TSX and shared UI/format SSR tests.
// Depends on: built-in module hooks and already-installed typescript-api; no build/dependency/source writes.
// Used by: import-history client and SSR tests only; no browser acceptance claim.
import { registerHooks } from "node:module";
import { readFileSync } from "node:fs";
import ts from "typescript-api";
/** Register repository-native extension resolution and CSS stubs for local Node tests. */
export function registerHistoryTestLoader() {
  return registerHooks({
    resolve(specifier, context, nextResolve) {
      // Native Node ESM needs the installed Next entry's extension; this still loads the real module.
      if (specifier === "next/headers") return nextResolve("next/headers.js", context);
      try { return nextResolve(specifier, context); } catch (error) {
        if (!specifier.startsWith(".")) throw error;
        for (const extension of [".ts", ".tsx"]) { try { return nextResolve(specifier + extension, context); } catch {} }
        throw error;
      }
    },
    load(url, context, nextLoad) {
      if (url.endsWith(".css")) return { format: "module", shortCircuit: true, source: "export default {};" };
      if (/\/(?:apps\/admin|packages\/(?:ui|format))\//.test(url) && /\.tsx?$/.test(url)) return {
        format: "module", shortCircuit: true, source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX }, fileName: new URL(url).pathname,
        }).outputText,
      };
      return nextLoad(url, context);
    },
  });
}
