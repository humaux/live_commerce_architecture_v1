// Purpose: Derive bulk and variant-remove action coverage from the actual production JSX, rather than the case table alone.
// Depends on: typescript-api, ProductBulkFill / ProductDocumentVariants source and the canonical mobile-matrix cases.
// Used by: Node gate; every rendered bulk control and every axis/value remove button needs a stable identity and a real click case.
import ts from 'typescript-api';
export function bulkControlIssues(source, cases) {
  const ast = ts.createSourceFile('ProductBulkFill.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const issues = [], seen = new Set();
  function visit(node) {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const tag = node.tagName.getText(ast);
      if (['select', 'input', 'button'].includes(tag)) {
        const attr = node.attributes.properties.find(a => ts.isJsxAttribute(a) && a.name.getText(ast) === 'data-testid');
        const id = attr?.initializer && ts.isStringLiteral(attr.initializer) ? attr.initializer.text : null;
        if (!id) issues.push(`Unidentified actionable ${tag}`);
        else {
          if (seen.has(id)) issues.push(`Duplicate actionable ${id}`);
          seen.add(id);
          const verb = { select: 'select', input: 'fill', button: 'click' }[tag];
          if (!cases.some(c => ['mobile matrix bulk', 'mobile matrix bulk code set', 'mobile matrix bulk compare set'].includes(c.page) && c.control === id && c.action.includes(verb)))
            issues.push(`Missing rendered ${tag} action ${id}`);
        }
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(ast);
  if (!seen.size) issues.push('No production bulk controls discovered');
  return issues;
}

/** Bind the actual DOM inventory call to the imported recorder's method, including lexical shadowing. */
export function bulkInventoryBindingIssues(source) {
  const filename = 'product-editor.acceptance.ts';
  const file = ts.createSourceFile(filename, source, ts.ScriptTarget.Latest, true);
  const host = ts.createCompilerHost({ noResolve: true, noLib: true });
  host.getSourceFile = name => name === filename ? file : undefined;
  const checker = ts.createProgram([filename], { noResolve: true, noLib: true }, host).getTypeChecker();
  const issues = [], calls = new Map();
  const states = new Map([
    ['create-price', 'mobile matrix bulk code set'], ['create-code', 'mobile matrix bulk code set'],
    ['edit-price', 'mobile matrix bulk'], ['edit-keyword', 'mobile matrix bulk'],
    ['edit-quantity', 'mobile matrix bulk'], ['edit-compare', 'mobile matrix bulk compare set'],
  ]);
  function visit(node) {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === 'matrixRendered') {
      const [page, inventory, state] = node.arguments;
      const name = state && ts.isStringLiteral(state) ? state.text : '';
      calls.set(name, (calls.get(name) ?? 0) + 1);
      if (node.arguments.length !== 3 || !page || !ts.isStringLiteral(page) || states.get(name) !== page.text)
        issues.push('Bulk DOM inventory requires its exact declared state/page');
      const inventoryBindings = inventory && ts.isIdentifier(inventory) ? checker.getSymbolAtLocation(inventory)?.declarations ?? [] : [];
      const inventorySource = inventoryBindings.length === 1 && ts.isVariableDeclaration(inventoryBindings[0]) ? inventoryBindings[0].initializer?.getText(file) : '';
      const expectedInventory = 'await page.getByTestId("bulk-dialog").locator("select, input, button").evaluateAll(nodes => nodes.map(node => ({ control: node.getAttribute("data-testid"), tag: node.tagName.toLowerCase(), disabled: (node as HTMLInputElement).disabled })))';
      if (inventorySource !== expectedInventory) issues.push('Bulk DOM inventory must sample actual control identity, tag and disabled state');
      const bindings = checker.getSymbolAtLocation(node.expression)?.declarations ?? [];
      const binding = bindings.length === 1 ? bindings[0] : undefined;
      const variable = binding?.parent?.parent, init = variable?.initializer;
      const imports = init && ts.isCallExpression(init) ? checker.getSymbolAtLocation(init.expression)?.declarations ?? [] : [];
      const imported = imports.length === 1 ? imports[0] : undefined;
      if (!binding || !ts.isBindingElement(binding) || binding.propertyName?.getText(file) !== 'assertRenderedControls' ||
          !init || !ts.isCallExpression(init) || init.arguments.map(arg => arg.getText(file)).join(',') !== 'ledger,locale' ||
          !imported || !ts.isImportSpecifier(imported) || (imported.propertyName ?? imported.name).text !== 'createProductEditorMatrixLedger' ||
          imported.parent.parent.parent.moduleSpecifier.text !== './product-editor-matrix-ledger')
        issues.push('DOM inventory must use the imported real recorder assertRenderedControls method');
    }
    ts.forEachChild(node, visit);
  }
  visit(file);
  if (calls.size !== states.size || [...states.keys()].some(state => calls.get(state) !== 1))
    issues.push('Exactly one real bulk DOM inventory call per required state must exist in the locale flow');
  return issues;
}

const escape = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
/** Every rendered remove control (aria-label with c.remove, or a data-testid naming "remove", on a button or role="button": #147 CI-11) needs a testid template and, per axis index, a real click case. */
export function removeControlIssues(source, cases) {
  const ast = ts.createSourceFile('ProductDocumentVariants.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  // The production axis cap (axis-add is disabled at this count) bounds "every axis index".
  const axes = Number(/axes\.length >= (\d+)/.exec(source)?.[1]);
  const issues = axes > 0 ? [] : ['Production axis limit not discovered'];
  let found = 0;
  const attr = (node, name) => node.attributes.properties.find(a => ts.isJsxAttribute(a) && a.name.getText(ast) === name)?.initializer;
  function visit(node) {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const tag = node.tagName.getText(ast), role = attr(node, 'role')?.getText(ast) ?? '';
      const label = attr(node, 'aria-label')?.getText(ast) ?? '', testId = attr(node, 'data-testid')?.getText(ast) ?? '';
      if ((tag === 'button' || /button/.test(role)) && (label.includes('${c.remove}') || /remove/i.test(testId))) {
        found++;
        const id = attr(node, 'data-testid'), template = id && ts.isJsxExpression(id) && id.expression && ts.isTemplateExpression(id.expression) ? id.expression : null;
        if (!template) issues.push(`Unidentified remove control ${label || node.getText(ast)}`);
        else for (let axis = 0; axis < axes; axis++) {
          // `${i}` is the axis index in both remove controls; any other span (the value index) may vary.
          const pattern = new RegExp('^' + escape(template.head.text) + template.templateSpans.map(span =>
            (span.expression.getText(ast) === 'i' ? axis : '\\d+') + escape(span.literal.text)).join('') + '$');
          if (!cases.some(c => pattern.test(c.control) && c.action.includes('click')))
            issues.push(`Missing rendered remove click ${pattern.source}`);
        }
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(ast);
  if (!found) issues.push('No production remove controls discovered');
  return issues;
}
