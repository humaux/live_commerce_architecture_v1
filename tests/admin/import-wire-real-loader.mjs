// Purpose: load actual import routes, clients and shared auth helpers for network-only Node seams.
// Depends on: existing history TS loader, native module hooks and installed Next react-server marker.
// Used by: import-client/import-history seam tests; no replacement of auth, cookies, CSRF or localError.
import {registerHooks,createRequire} from "node:module";
import {registerHistoryTestLoader} from "./import-history-test-loader.mjs";
const requireApp=createRequire(new URL("../../apps/admin/package.json",import.meta.url));
/** Resolve Next's actual server marker and repository aliases; every business/shared helper stays real. */
export function registerImportWireLoader(){
  const types=registerHistoryTestLoader();
  const aliases=registerHooks({resolve(specifier,context,nextResolve){
    if(specifier==="server-only")return nextResolve(requireApp.resolve("next/dist/compiled/server-only/empty"),context);
    if(specifier.startsWith("@/"))return nextResolve(new URL(`../../apps/admin/${specifier.slice(2)}.ts`,import.meta.url).href,context);
    return nextResolve(specifier,context);
  }});
  return {deregister(){aliases.deregister();types.deregister();}};
}
