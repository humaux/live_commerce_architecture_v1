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
          if (!cases.some(c => c.page === 'mobile matrix bulk' && c.control === id && c.action.includes(verb)))
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
  const issues = []; let calls = 0;
  function visit(node) {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === 'matrixRendered') {
      calls++;
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
  if (calls !== 1) issues.push('Exactly one real bulk DOM inventory call must exist in the locale flow');
  return issues;
}

const escape = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
/** Every rendered remove button (aria-label with c.remove, or a data-testid naming "remove": #147 CI-11) needs a testid template and, per axis index, a real click case. */
export function removeControlIssues(source, cases) {
  const ast = ts.createSourceFile('ProductDocumentVariants.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  // The production axis cap (axis-add is disabled at this count) bounds "every axis index".
  const axes = Number(/axes\.length >= (\d+)/.exec(source)?.[1]);
  const issues = axes > 0 ? [] : ['Production axis limit not discovered'];
  let found = 0;
  const attr = (node, name) => node.attributes.properties.find(a => ts.isJsxAttribute(a) && a.name.getText(ast) === name)?.initializer;
  function visit(node) {
    if ((ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) && node.tagName.getText(ast) === 'button' &&
        ((attr(node, 'aria-label')?.getText(ast) ?? '').includes('${c.remove}') || /remove/i.test(attr(node, 'data-testid')?.getText(ast) ?? ''))) {
      found++;
      const id = attr(node, 'data-testid'), template = id && ts.isJsxExpression(id) && id.expression && ts.isTemplateExpression(id.expression) ? id.expression : null;
      if (!template) issues.push(`Unidentified remove control ${attr(node, 'aria-label')?.getText(ast) ?? node.getText(ast)}`);
      else for (let axis = 0; axis < axes; axis++) {
        // `${i}` is the axis index in both remove controls; any other span (the value index) may vary.
        const pattern = new RegExp('^' + escape(template.head.text) + template.templateSpans.map(span =>
          (span.expression.getText(ast) === 'i' ? axis : '\\d+') + escape(span.literal.text)).join('') + '$');
        if (!cases.some(c => pattern.test(c.control) && c.action.includes('click')))
          issues.push(`Missing rendered remove click ${pattern.source}`);
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(ast);
  if (!found) issues.push('No production remove controls discovered');
  return issues;
}
