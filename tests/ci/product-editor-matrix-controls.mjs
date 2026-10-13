// Purpose: Derive bulk action coverage from the actual production JSX, rather than the case table alone.
// Depends on: typescript-api, ProductBulkFill source and the canonical mobile-matrix cases.
// Used by: Node gate; every rendered select/input/button needs a stable identity and real matching operation.
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
