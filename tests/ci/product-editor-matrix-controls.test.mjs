// Purpose: Refuse bulk and variant-remove completeness that merely agrees with an incomplete declaration table.
// Depends on: the production JSX scanner and actual ProductBulkFill/ProductDocumentVariants/canonical table bytes.
// Used by: test-node; mutations must turn omissions into RED without a browser or PG.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript-api';
import { bulkControlIssues, bulkInventoryBindingIssues, removeControlIssues } from './product-editor-matrix-controls.mjs';
const source = readFileSync('apps/admin/components/ProductBulkFill.tsx', 'utf8');
const cases = JSON.parse(readFileSync('tests/foundation/testdata/product_editor_matrix_cases.json', 'utf8'));
test('actual rendered bulk controls have matching real actions in the canonical table', () => {
  assert.deepEqual(bulkControlIssues(source, cases), []);
});
test('removing the Apply declaration is RED even when table and ledger agree', () => {
  assert.ok(bulkControlIssues(source, cases.filter(c => c.control !== 'bulk-apply')).some(x => x.includes('bulk-apply')));
});
test('a new rendered control without a case is RED', () => {
  const added = source.replace('</div>', '<button data-testid="bulk-new">New</button></div>');
  assert.ok(bulkControlIssues(added, cases).some(x => x.includes('bulk-new')));
});
test('Escape alone is not a select action', () => {
  const changed = cases.map(c => c.control === 'bulk-field' ? { ...c, action: 'Escape' } : c);
  assert.ok(bulkControlIssues(source, changed).some(x => x.includes('bulk-field')));
});
test('changing the production identity is RED', () => {
  assert.ok(bulkControlIssues(source.replace('data-testid="bulk-field"', 'data-testid="bulk-other"'), cases).some(x => x.includes('bulk-other')));
});

function loadTS(path, require = () => { throw new Error('Unexpected dependency'); }) {
  const module = { exports: {} };
  vm.runInNewContext(ts.transpileModule(readFileSync(path, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, { module, exports: module.exports, require });
  return module.exports;
}
const copy = loadTS('apps/admin/lib/product-editor-copy.ts');
const { createProductEditorMatrixLedger } = loadTS('tests/admin/product-editor-matrix-ledger.ts', name => name.endsWith('product_editor_matrix_cases.json') ? cases : copy);
const rendered = [];
const jsx = ts.createSourceFile('bulk.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
function visit(node) {
  if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
    const tag = node.tagName.getText(jsx);
    if (['select', 'input', 'button'].includes(tag)) {
      const attr = node.attributes.properties.find(a => ts.isJsxAttribute(a) && a.name.getText(jsx) === 'data-testid');
      rendered.push({ control: attr?.initializer?.text ?? null, tag });
    }
  }
  ts.forEachChild(node, visit);
}
visit(jsx);
test('real recorder admits the actual rendered bulk inventory in all three locales', () => {
  for (const locale of ['zh-TW', 'zh-CN', 'en'])
    createProductEditorMatrixLedger([], locale).assertRenderedControls('mobile matrix bulk', rendered);
});
test('real recorder refuses an undeclared or anonymous production control and duplicates', () => {
  const recorder = createProductEditorMatrixLedger([], 'en');
  for (const extra of [{ control: 'bulk-new', tag: 'button' }, { control: null, tag: 'button' }, rendered[0]])
    assert.throws(() => recorder.assertRenderedControls('mobile matrix bulk', [...rendered, extra]), /Missing rendered|Unidentified or duplicate/);
  assert.throws(() => recorder.assertRenderedControls('mobile matrix bulk', []), /No rendered/);
});

const spec = readFileSync('tests/admin/product-editor.acceptance.ts', 'utf8');
test('actual DOM inventory is bound to the real imported recorder method', () => {
  assert.deepEqual(bulkInventoryBindingIssues(spec), []);
});
test('a swapped method or no-op shadow cannot donate a DOM inventory assertion', () => {
  assert.ok(bulkInventoryBindingIssues(spec.replace('assertRenderedControls: matrixRendered', 'assertComplete: matrixRendered')).length);
  assert.ok(bulkInventoryBindingIssues(spec.replace('matrixRendered("mobile matrix bulk", renderedBulkControls);', 'const matrixRendered = () => {}; matrixRendered("mobile matrix bulk", renderedBulkControls);')).length);
});

const variants = readFileSync('apps/admin/components/ProductDocumentVariants.tsx', 'utf8');
test('every rendered axis and value remove control has a real click case for every axis index', () => {
  assert.deepEqual(removeControlIssues(variants, cases), []);
});
test('removing any one remove-control case is RED', () => {
  const removes = cases.filter(c => /^axis-(value-)?remove-/.test(c.control));
  for (const dropped of removes)
    assert.ok(removeControlIssues(variants, cases.filter(c => c !== dropped)).length, `${dropped.page} ${dropped.control} must be required`);
});
test('an anonymous or new remove control is RED', () => {
  const anonymous = variants.replace('data-testid={`axis-value-remove-${i}-${n}`}', '');
  assert.ok(removeControlIssues(anonymous, cases).some(x => x.includes('Unidentified remove control')));
  const added = variants.replace('data-testid={`axis-remove-${i}`}', 'data-testid={`axis-drop-${i}`}');
  assert.ok(removeControlIssues(added, cases).some(x => x.includes('axis-drop-')));
});
// #147 CI-11: a remove button is inventoried by its testid as well, so a different aria-label cannot hide it.
test('a remove control with another aria-label is still inventoried by its testid', () => {
  const relabeled = variants.replace('aria-label={`${c.remove} ${value}`}', 'aria-label={`Delete ${value}`}');
  assert.notEqual(relabeled, variants);
  const without = cases.filter(c => !c.control.startsWith('axis-value-remove-'));
  assert.ok(removeControlIssues(relabeled, without).some(x => x.includes('axis-value-remove-')));
});
// #147 CI-11: role="button" elements are remove controls too; the locale label is not the identity.
test('a role="button" remove control is inventoried without the locale remove aria-label', () => {
  const start = variants.search(/<button\s+className="pe-axis-remove"/);
  assert.ok(start >= 0);
  const end = variants.indexOf('</button>', start) + '</button>'.length;
  const block = variants.slice(start, end)
    .replace(/<button(\s+)className="pe-axis-remove"/, '<span$1role="button"$1className="pe-axis-remove"')
    .replace('aria-label={`${c.remove} ${c.axis} ${i + 1}`}', 'aria-label={`Delete axis ${i + 1}`}')
    .replace('</button>', '</span>');
  const roleRemove = variants.slice(0, start) + block + variants.slice(end);
  assert.notEqual(roleRemove, variants);
  const without = cases.filter(c => !c.control.startsWith('axis-remove-'));
  assert.ok(removeControlIssues(roleRemove, without).some(x => x.includes('axis-remove-')), 'role="button" remove control must be inventoried by its testid');
});
