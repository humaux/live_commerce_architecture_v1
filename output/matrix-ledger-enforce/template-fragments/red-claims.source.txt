// Purpose: Compare reached matrix assertions with independently declared canonical expected-claim requirements.
// Depends on: TypeScript AST/type checker and the shared 77-case declaration; no browser, PG or network calls.
// Used by: product-editor-matrix-source.mjs; exact matcher operands, binding provenance and reload ownership are required.
import assert from "node:assert/strict";
import ts from "typescript-api";

// A source-only clock makes generated operands exact and repeatable. The real
// browser retains Date.now(); no runtime fixture, wait or assertion is changed.
export const sourceClock = 1770001234567;

// Structural keys preserve literal values/operators while ignoring formatting and harmless type/parenthesis wrappers.
const key = (node) => {
  if (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) || ts.isNonNullExpression(node)) return key(node.expression);
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return ["string", node.text];
  if (ts.isNumericLiteral(node)) return ["number", Number(node.text)];
  if (ts.isIdentifier(node)) return ["identifier", node.text];
  if (ts.isRegularExpressionLiteral(node)) return ["regexp", node.text];
  const children = []; ts.forEachChild(node, (child) => { children.push(key(child)); });
  return [ts.SyntaxKind[node.kind], ...children];
};
const signature = (node) => JSON.stringify(key(node));
const parse = (expression) => {
  const source = ts.createSourceFile("canonical-claim.ts", expression, ts.ScriptTarget.Latest, true);
  assert.equal(source.parseDiagnostics.length, 0, "invalid canonical assertion expression");
  assert.equal(source.statements.length, 1, "one canonical expression required");
  assert.ok(ts.isExpressionStatement(source.statements[0]), "canonical claim must be an expression");
  return source.statements[0].expression;
};
const walk = (node, visit) => { visit(node); ts.forEachChild(node, (child) => walk(child, visit)); };
const expand = (value, context) => {
  if (typeof value === "string") return value.replace(/@(?:\{([A-Za-z]+)\}|([A-Za-z]+))/g, (_, braced, plain) => {
    const token = braced ?? plain;
    assert.ok(token in context, `unknown canonical operand token ${token}`); return context[token];
  });
  if (Array.isArray(value)) return value.map((child) => expand(child, context));
  if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, expand(v, context)]));
  return value;
};
const equivalent = (actual, wanted) => {
  if (wanted && typeof wanted === "object" && !Array.isArray(wanted) && "regexp" in wanted)
    return actual && actual.regexp === new RegExp(wanted.regexp, wanted.flags).source && actual.flags === wanted.flags;
  if (wanted && typeof wanted === "object" && !Array.isArray(wanted) && "pattern" in wanted)
    return typeof actual === "string" && new RegExp(wanted.pattern).test(actual);
  if (Array.isArray(wanted)) return Array.isArray(actual) && actual.length === wanted.length && wanted.every((v, i) => equivalent(actual[i], v));
  if (wanted && typeof wanted === "object") return actual && typeof actual === "object" &&
    Object.keys(actual).length === Object.keys(wanted).length && Object.entries(wanted).every(([k, v]) => equivalent(actual[k], v));
  return actual === wanted;
};

/** Builds a source proof tracer and validates each actual callback against its canonical expected result. */
export function canonicalClaims(source, checker, steps, cases, copy) {
  const statements = new Map(), callbacks = new Map();
  for (const [callback, step] of steps) {
    callbacks.set(callback, step);
    const visit = (node) => {
      if (node !== callback && ts.isFunctionLike(node)) return;
      if (ts.isExpressionStatement(node)) statements.set(node, signature(node.expression));
      ts.forEachChild(node, visit);
    };
    visit(callback.body);
  }
  const definitions = (callback) => {
    const bindings = new Map(), seen = new Set();
    const follow = (node, invoked = false) => {
      // Declarations and unused closures cannot donate a same-name proof binding.
      if (ts.isFunctionLike(node) && !invoked) return;
      if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "map")
        for (const argument of node.arguments) if (ts.isArrowFunction(argument)) follow(argument, true);
      if (ts.isIdentifier(node) && !(ts.isVariableDeclaration(node.parent) && node.parent.name === node)) {
        for (const decl of checker.getSymbolAtLocation(node)?.declarations ?? []) {
          let variable = decl;
          while (ts.isBindingElement(variable) || ts.isArrayBindingPattern(variable) || ts.isObjectBindingPattern(variable)) variable = variable.parent;
          if (!ts.isVariableDeclaration(variable) || seen.has(variable)) continue;
          const iteration = variable.parent?.parent;
          const initializer = variable.initializer ?? (ts.isForOfStatement(iteration) ? iteration.expression : undefined);
          if (!initializer) continue;
          seen.add(variable);
          if (ts.isIdentifier(variable.name)) {
            const prior = bindings.get(node.text) ?? [];
            bindings.set(node.text, [...prior, variable]);
          }
          follow(initializer, ts.isCallExpression(node.parent) && node.parent.expression === node);
        }
      }
      ts.forEachChild(node, follow);
    };
    follow(callback.body); return bindings;
  };
  const verify = (entry, declaration, bindingStages, regionStart) => {
    const required = declaration.requirements;
    assert.ok(required, "canonical expected claim has no machine requirements");
    const fail = (detail) => assert.fail(`canonical expected claim ${declaration.expected}: ${detail}`);
    const context = { ...copy[entry.locale], tag: "MOCK-source-gate", matrixTag: `M${String(sourceClock).slice(-10)}`, locale: entry.locale,
      localeIndex: { "zh-TW": 0, "zh-CN": 1, en: 2 }[entry.locale] };
    for (const wanted of required.ui ?? []) {
      const pattern = expand(wanted, context);
      const matches = entry.claimUI.filter((actual) => equivalent(actual.receiver, pattern.locator) && actual.matcher === pattern.matcher &&
        actual.negated === (pattern.negated ?? false) && equivalent(actual.args, pattern.args ?? []) &&
        (pattern.action === undefined || actual.action === (pattern.action === "last" ? entry.actions.length : pattern.action)) &&
        (pattern.reload === undefined || actual.reload === pattern.reload));
      if (matches.length < (pattern.count ?? 1)) fail(`missing UI matcher/operand ${JSON.stringify(pattern)}`);
    }
    for (const wanted of required.facts ?? []) {
      const expression = signature(parse(wanted.expression));
      const matches = entry.claimFacts.filter((actual) => actual.signature === expression &&
        actual.action === (wanted.action === undefined || wanted.action === "last" ? entry.actions.length : wanted.action) &&
        (wanted.reload === undefined || actual.reload === wanted.reload));
      if (matches.length !== (wanted.count ?? 1)) fail(`missing reached assertion ${wanted.expression} at reload ${wanted.reload ?? "any"}`);
    }
    if ((entry.reloads ?? 0) !== (required.reloads ?? 0)) fail(`reload count ${entry.reloads}; expected ${required.reloads ?? 0}`);
    if (required.persisted && !(entry.parentReload > 0)) fail("readback is not after its parent's save/reload");
    for (const child of required.children ?? []) {
      if (!entry.children.some((actual) => actual.identity.page === entry.identity.page && actual.identity.control === child.control &&
          (actual.identity.row ?? "") === (child.row ?? "") && actual.reload === child.reload)) fail(`missing persisted child ${JSON.stringify(child)}`);
    }
    const callback = entry.callback, found = definitions(callback);
    for (const [name, expected] of Object.entries(required.bindings ?? {})) {
      const bindings = found.get(name) ?? [];
      const expression = typeof expected === "string" ? expected : expected.expression;
      if (!bindings.length || bindings.some((binding) => {
        const loop = binding.parent?.parent;
        const initializer = binding.initializer ?? (ts.isForOfStatement(loop) ? loop.expression : undefined);
        return !initializer || signature(initializer) !== signature(parse(expression)) ||
          (typeof expected === "object" && expected.iteration && !ts.isForOfStatement(loop));
      })) fail(`binding ${name} has the wrong value/source`);
      for (const binding of bindings) {
      if (typeof expected === "object" && expected.beforeAction && entry.firstOperation && binding.getEnd() >= entry.firstOperation.getStart(source))
        fail(`binding ${name} must capture before the action`);
      if (typeof expected === "object" && expected.outsideRegion && binding.getStart(source) >= regionStart)
        fail(`binding ${name} must use the real preexisting observation collector`);
      if (typeof expected === "object" && expected.reload !== undefined &&
          bindingStages.get(binding.pos)?.revision - entry.beginRevision !== expected.reload)
        fail(`binding ${name} must be read at reload ${expected.reload}`);
      if (typeof expected === "object" && expected.afterAction && !(bindingStages.get(binding.pos)?.action > 0))
        fail(`binding ${name} must be read after the action`);
      }
    }
  };
  return { statements, verify, signature };
}
