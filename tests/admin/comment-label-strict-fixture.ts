// Purpose: real installed React development StrictMode fixture for the production label component.
// Depends on: installed React/DOM/scheduler CJS builds, typescript-api and the actual component source; transport is MOCK only.
// Used by: comment-label-print browser spec; no product route, new dependency or simulated React hook runtime.
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
const require = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const ts = require("typescript-api");

/** Builds in-memory test assets; React itself and the component are never replaced by hook stubs. */
export function commentLabelStrictFixture() {
  const packageSource = (name: string, file: string) => {
    const resolver = name === "scheduler" ? createRequire(require.resolve("react-dom/client")) : require;
    return readFileSync(join(dirname(resolver.resolve(`${name}/package.json`)), "cjs", file), "utf8");
  };
  const component = readFileSync(new URL("../../apps/admin/components/CommentLabelPrint.tsx", import.meta.url), "utf8");
  const modules: Record<string, string> = {
    react: packageSource("react", "react.development.js"),
    "react/jsx-runtime": packageSource("react", "react-jsx-runtime.development.js"),
    "react-dom": packageSource("react-dom", "react-dom.development.js"),
    "react-dom/client": packageSource("react-dom", "react-dom-client.development.js"),
    scheduler: packageSource("scheduler", "scheduler.development.js"),
    "@live-commerce/format": "exports.displayTime=(_,value)=>value;",
    "@/app/print.css": "",
    "@/lib/inbox-client": `exports.InboxError=class extends Error{};
      exports.inboxWrite=(_,path,body,key,signal)=>{
        if(signal.aborted)throw new Error("Aborted StrictMode print controller");
        window.__strictLabel.writes++;
        return new Promise(resolve=>{window.__resolveStrictPrint=()=>resolve({print_count:1,last_printed_at:null});});
      };`,
    component: ts.transpileModule(component, { compilerOptions: {
      target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX,
    } }).outputText,
  };
  const factories = Object.entries(modules).map(([name, source]) => `${JSON.stringify(name)}:function(module,exports,require){\n${source}\n}`).join(",\n");
  const script = `(()=>{
    const process={env:{NODE_ENV:"development"}},factories={${factories}},cache={};
    function require(name){if(cache[name])return cache[name].exports;
      if(!factories[name])throw new Error("Unknown StrictMode fixture module: "+name);
      const module={exports:{}};cache[name]=module;factories[name](module,module.exports,require);return module.exports;}
    const React=require("react"),C=require("component");
    const stats=window.__strictLabel={setups:0,cleanups:0,writes:0,prints:0,version:React.version};
    window.print=()=>{stats.prints++;};
    function Probe(){React.useEffect(()=>{stats.setups++;return()=>{stats.cleanups++;};},[]);return null;}
    const row={ref:"123_456",seq:1,parent_ref:null,author_name:"SYNTHETIC",created_at:"2030-01-01T00:00:00Z",
      text:"A1 1",is_page:false,has_attachment:false,marks:{intake:null,private_reply:null,printed:null,public_replies:0,private_reply_available:false,
      claim:{status:"ACCEPTED",reason:null,offer_id:"11111111-1111-4111-8111-111111111111",keyword:"A1",quantity:1,bundle_id:"33333333-3333-4333-8333-333333333333"}}};
    require("react-dom/client").createRoot(document.getElementById("root")).render(
      React.createElement(React.StrictMode,null,React.createElement(Probe),React.createElement(C.CommentLabelPrint,
        {locale:"en",store:"11111111-1111-4111-8111-111111111111",session:"22222222-2222-4222-8222-222222222222",rows:[row],active:true,allowed:true,onDenied(){throw new Error("Unexpected fixture denial");}},
        React.createElement(C.CommentLabelAction,{row}))));
  })();`;
  return { script, html: '<!doctype html><html><head><meta charset="utf-8"><title>MOCK React development lifecycle</title></head><body><div id="root"></div><script src="/__test/w3u3-strict.js"></script></body></html>' };
}
