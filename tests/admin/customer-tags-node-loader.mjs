// Purpose: focused Node test loader for the repository's TSX and extensionless client imports.
// Depends on: built-in module hooks and installed TypeScript; compiles only in memory.
// Used by: customer-tags UI/transport tests; never changes product source or adds dependencies.
import { registerHooks } from "node:module";
import { readFileSync } from "node:fs";
import ts from "typescript-api";

/** Register in-memory TSX/TS loading and CSS stubs for SSR-only tests. */
export function registerCustomerTagsLoader() {
  return registerHooks({
    resolve(specifier, context, nextResolve) {
      try { return nextResolve(specifier, context); }
      catch (error) {
        if (!specifier.startsWith(".")) throw error;
        for (const extension of [".ts", ".tsx", ".js"]) {
          try { return nextResolve(specifier + extension, context); } catch { /* Try the next repository-native source extension. */ }
        }
        throw error;
      }
    },
    load(url, context, nextLoad) {
      if (url.endsWith(".css")) return { format: "module", shortCircuit: true, source: "export default {};" };
      if (url.includes("/apps/admin/") && /\.tsx?$/.test(url)) return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX }, fileName: new URL(url).pathname,
        }).outputText,
      };
      return nextLoad(url, context);
    },
  });
}
