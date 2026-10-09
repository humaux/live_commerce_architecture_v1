// Purpose: causal counterexamples for browser evidence write-root hygiene.
// Depends on: real static checker and synthetic source snippets; no application behavior is mocked.
// Used by: test-node.sh; historical reads stay legal while regression writes fail.
import test from 'node:test';
import assert from 'node:assert/strict';
import {inspectEvidenceSource,browserProducer} from '../../scripts/dev/check-browser-evidence.mjs';
const file='tests/admin/example.spec.ts';
const bad=source=>inspectEvidenceSource(file,source,{tracked:['.impeccable/review/hero-repro.png']});
for(const [name,source] of [
 ['direct',`await writeFile('output/product-ui-v2/ledger.json','x')`],
 ['alias',`const out=path.resolve('output/product-ui-v2'); const file=path.join(out,'x.png'); await page.screenshot({path:file})`],
 ['split join',`const out=path.join(root,'output','product-ui-v2');await mkdir(out)`],
 ['template', 'const out=`output/product-ui-v2/${phase}`;await writeFile(`${out}/x.json`,value)'],
 ['helper',`function legacy(){return path.join(root,'output','product-ui-v2')};await writeFile(legacy(),'x')`],
 ['copy destination',`await copyFile('output/playwright/run/image.png','output/product-ui-v2/image.png')`],
 ['reporter',`export default defineConfig({reporter:[['json',{outputFile:'output/product-ui-v2/result.json'}]]})`],
 ['traversal',`await writeFile(path.join('output/playwright','..','product-ui-v2','x'),'x')`],
 ['tracked nonoutput',`await page.screenshot({path:'.impeccable/review/hero-repro.png'})`],
])test(`reject ${name}`,()=>assert.ok(bad(source).length,source));
for(const [name,source] of [
 ['historical read',`const baseline=path.join(root,'output','product-ui-v2','before.json'); const data=await readFile(baseline); await writeFile(path.join(evidence,'after.json'),data)`],
 ['copy source',`await copyFile('output/product-ui-v2/before.png','output/playwright/run/before.png')`],
 ['comment',`// await writeFile('output/product-ui-v2/x','x')\nawait writeFile('output/playwright/run/x','x')`],
 ['string data',`await writeFile('output/playwright/run/x',"writeFile('output/product-ui-v2/x','data')")`],
 ['run directory',`const out=path.resolve('output/playwright',run); await mkdir(out); await page.screenshot({path:path.join(out,'x.png')})`],
])test(`allow ${name}`,()=>assert.deepEqual(bad(source),[]));
test('Go Join aliases flag writes but not read-only history or comments',()=>{
 const source=`package example\nfunc test(){\n base := filepath.Join(root, "output", "product-ui-v2")\n dir := filepath.Join(base, "run")\n os.WriteFile(filepath.Join(dir,"x.json"),data,0600)\n}`;
 assert.ok(inspectEvidenceSource('tests/foundation/browser_example_test.go',source).length);
 assert.deepEqual(inspectEvidenceSource('tests/foundation/browser_example_test.go',source.replace('os.WriteFile','os.ReadFile')),[]);
 assert.deepEqual(inspectEvidenceSource('tests/foundation/browser_example_test.go',source.replace('"output", "product-ui-v2"','"output", "playwright"')),[]);
});
test('producer discovery covers acceptance suites and helpers, not documentation/artifacts',()=>{
 for(const p of ['tests/admin/product-editor.acceptance.ts','tests/admin/home-cod.spec.ts','tests/storefront/order-gate.mjs','tests/foundation/browser_example_test.go','tests/ui/visual-audit.mjs'])assert.ok(browserProducer(p));
 for(const p of ['docs/delivery/GATES.md','output/old/test.spec.ts','tests/ci/browser-evidence-paths.test.mjs','tests/ui/sweep-aggregate.mjs'])assert.equal(browserProducer(p),false);
});

test('synthetic Go historical fixtures stay inside t.TempDir; real CopyFS destinations are audited',()=>{
 const fake='package example\nfunc test(){ root := t.TempDir()\n os.WriteFile(filepath.Join(root,"output","product-ui-v2","x"),data,0600)\n}';
 assert.deepEqual(inspectEvidenceSource('tests/foundation/browser_paths_test.go',fake),[]);
 assert.ok(inspectEvidenceSource('tests/foundation/browser_paths_test.go','os.CopyFS(filepath.Join(root,"output","product-ui-v2"),fsys)').length);
});
