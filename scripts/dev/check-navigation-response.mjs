// Purpose: flag a path-ambiguous response used as the receipt for a specific browser navigation.
// Depends on: the pinned TypeScript parser.
// Used by: check-browser-spec-types.mjs for TypeScript and JavaScript browser tests.
import ts from "typescript-api";

function receiver(node, method) {
  return node && ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) &&
    node.expression.name.text === method ? node.expression.expression : undefined;
}

function named(node, name) {
  return !!node && ts.isIdentifier(node) && node.text === name;
}

function literal(node) {
  return node && (ts.isStringLiteralLike(node) ? node.text : ts.isTemplateExpression(node) ? node.head.text : undefined);
}

// Admit only predicates made entirely of method/document/frame checks or a pathname suffix.
// Helpers, regexes, exact URLs, response type guards and other constraints stay outside this check.
function unbound(node, response, page) {
  if (ts.isBlock(node)) {
    const [statement] = node.statements;
    return node.statements.length === 1 && ts.isReturnStatement(statement) && statement.expression &&
      unbound(statement.expression, response, page);
  }
  if (ts.isParenthesizedExpression(node)) return unbound(node.expression, response, page);
  if (ts.isBinaryExpression(node)) {
    if (node.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken)
      return unbound(node.left, response, page) && unbound(node.right, response, page);
    if (node.operatorToken.kind !== ts.SyntaxKind.EqualsEqualsEqualsToken) return false;
    const { left, right } = node;
    if (named(receiver(left, "frame"), response)) return named(receiver(right, "mainFrame"), page);
    for (const [method, value] of [["method", "GET"], ["resourceType", "document"]]) {
      if (named(receiver(receiver(left, method), "request"), response) &&
          ts.isStringLiteralLike(right) && right.text === value) return true;
    }
    return false;
  }
  const pathname = receiver(node, "endsWith");
  if (!pathname || !ts.isPropertyAccessExpression(pathname) || pathname.name.text !== "pathname") return false;
  const url = pathname.expression;
  return ts.isNewExpression(url) && named(url.expression, "URL") &&
    named(receiver(url.arguments?.[0], "url"), response) && literal(node.arguments[0])?.startsWith("/");
}

function pairedNavigation(wait, page) {
  const array = wait.parent;
  if (!ts.isArrayLiteralExpression(array) || !named(receiver(array.parent, "all"), "Promise")) return false;
  const awaited = array.parent.parent;
  if (!ts.isAwaitExpression(awaited) || !ts.isVariableDeclaration(awaited.parent) ||
      !ts.isArrayBindingPattern(awaited.parent.name)) return false;
  const binding = awaited.parent.name.elements[array.elements.indexOf(wait)];
  return !!binding && !ts.isOmittedExpression(binding) &&
    array.elements.some(element => named(receiver(element, "waitForURL"), page));
}

function linkThenJSON(wait, page) {
  const decl = wait.parent;
  if (!ts.isVariableDeclaration(decl) || !ts.isIdentifier(decl.name)) return false;
  const statement = decl.parent.parent;
  if (!ts.isVariableStatement(statement) || !ts.isBlock(statement.parent)) return false;
  const stmts = statement.parent.statements;
  const idx = stmts.indexOf(statement);
  const action = stmts[idx + 1], read = stmts[idx + 2];
  if (!action || !read || !ts.isExpressionStatement(action) || !ts.isAwaitExpression(action.expression)) return false;
  const link = receiver(action.expression.expression, "click");
  if (!link || !ts.isCallExpression(link)) return false;
  const selector = literal(link.arguments[0]);
  const linkRoot = receiver(link, "locator") ?? receiver(link, "getByRole");
  if (!linkRoot || !(receiver(link, "locator") ? /^a(?:\[href\b|$)/i.test(selector ?? "") : selector === "link")) return false;
  let root = linkRoot;
  while (ts.isCallExpression(root) && ts.isPropertyAccessExpression(root.expression)) root = root.expression.expression;
  if (!named(root, page)) return false;
  function readsPending(node) {
    let target = receiver(node, "json");
    while (target && ts.isParenthesizedExpression(target)) target = target.expression;
    return (target && ts.isAwaitExpression(target) && named(target.expression, decl.name.text)) ||
      ts.forEachChild(node, readsPending);
  }
  return readsPending(read);
}

export function navigationResponses(source) {
  const findings = [];
  function visit(node) {
    const page = receiver(node, "waitForResponse");
    if (page && ts.isIdentifier(page)) {
      const predicate = node.arguments[0];
      if (predicate && ts.isArrowFunction(predicate) && predicate.parameters.length === 1 &&
          ts.isIdentifier(predicate.parameters[0].name) &&
          unbound(predicate.body, predicate.parameters[0].name.text, page.text) &&
          (pairedNavigation(node, page.text) || linkThenJSON(node, page.text))) {
        const { line, character } = source.getLineAndCharacterOfPosition(node.getStart(source));
        findings.push(`${source.fileName}:${line + 1}:${character + 1} Navigation response can match a different path. Fix: match the expected document URL or the full API pathname in waitForResponse before reading its receipt.`);
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
  return findings;
}
