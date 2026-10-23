// Purpose: #179 lexical ratchet for browser gate scripts — a waitForResponse/waitForRequest (or the historyRead wrapper)
//   assigned to a variable must be awaited before the next top-level await. Otherwise an intervening assertion throws, the
//   page closes, the stray wait rejects unhandled and node exits with "Target page ... has been closed", hiding the real
//   first failure (tests/storefront/order-gate.mjs:738 before this fix; see issue #179).
// Depends on: nothing (plain text scan); callers pass {path, text} pairs and injected counterexamples.
// Used by: tests/ci/gate-waits.test.mjs; no browser, network or application code is executed.

// Assignment to NAME of a Playwright wait, directly or through the historyRead helper. A receiver chain such as
// `card.page().waitForResponse(` belongs to the call. An arrow factory (`const q = () => page.waitForResponse(`) has no
// NAME = call shape and stays out of scope: the factory's caller awaits the promise it returns.
const WAIT = /([A-Za-z_$][\w$]*)\s*=\s*((?:[A-Za-z_$][\w$]*(?:\([^()]*\))?\.)*)(historyRead|waitForResponse|waitForRequest)\s*\(/g;

const OPENERS = new Set(["(", "[", "{", "`"]);
const CLOSERS = new Set([")", "]", "}", "`"]);

// Index just past the balanced call whose opening parenthesis is at `open`.
function callEnd(text, open) {
  let depth = 0;
  for (let i = open; i < text.length; i++) {
    if (text[i] === "(") depth++;
    else if (text[i] === ")") { depth--; if (depth === 0) return i + 1; }
  }
  return -1;
}

// First `await` after `from` at bracket depth 0: an await inside a later callback/parenthesis is not the next statement.
function nextAwait(text, from) {
  let depth = 0;
  for (let i = from; i < text.length; i++) {
    const c = text[i];
    if (OPENERS.has(c)) depth++;
    else if (CLOSERS.has(c)) depth = Math.max(0, depth - 1);
    else if (depth === 0 && c === "a" && /^await\b/.test(text.slice(i, i + 5))) return i;
  }
  return -1;
}

// End of the await statement: first `;` or newline at depth 0 (a Promise.all span keeps its newlines inside brackets).
function statementEnd(text, from) {
  let depth = 0;
  for (let i = from; i < text.length; i++) {
    const c = text[i];
    if (OPENERS.has(c)) depth++;
    else if (CLOSERS.has(c)) depth = Math.max(0, depth - 1);
    else if (depth === 0 && (c === ";" || c === "\n")) return i;
  }
  return text.length;
}

const awaitedDirectly = (name) => new RegExp(`await\\s*\\(*\\s*${name}\\b`);
const AGGREGATE = /^await\s+Promise\.(?:all|allSettled|race)\(/;

// files: [{path, text}]. Returns one human-readable finding per wait that can reject unhandled.
export function offenders(files) {
  const found = [];
  for (const { path, text } of files) {
    const re = new RegExp(WAIT.source, "g");
    let match;
    while ((match = re.exec(text))) {
      const [whole, name] = match;
      const end = callEnd(text, match.index + whole.length - 1);
      if (end < 0) continue;
      const line = text.slice(0, match.index).split("\n").length;
      const at = nextAwait(text, end);
      if (at < 0) {
        found.push(`${path}:${line} ${name} is never awaited`);
        continue;
      }
      const statement = text.slice(at, statementEnd(text, at + 5));
      const direct = awaitedDirectly(name).test(statement);
      const aggregate = AGGREGATE.test(statement) && new RegExp(`\\b${name}\\b`).test(statement);
      if (!direct && !aggregate) {
        found.push(`${path}:${line} ${name} is awaited only after another await: ${statement.split("\n")[0].slice(0, 120)}`);
      }
    }
  }
  return found;
}
