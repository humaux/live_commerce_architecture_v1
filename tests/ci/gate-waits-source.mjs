// Purpose: #179 lexical ratchet for browser gate scripts — a waitForResponse/waitForRequest (or the historyRead wrapper)
//   assigned to a variable must be awaited before the next top-level await. Otherwise an intervening assertion throws, the
//   page closes, the stray wait rejects unhandled and node exits with "Target page ... has been closed", hiding the real
//   first failure (tests/storefront/order-gate.mjs:738 before this fix; see issue #179).
//   Round 2 also pins ORDERING: the wait has to be registered before the action that triggers the request. A wait awaited
//   inline (`const r = await page.waitForResponse(...)`) can only miss the reply or hang, and a `Promise.all([trigger(),
//   pending])` that lists the trigger first lets a fixture-held reply (arm(..., {after: true})) wait for a release that
//   lives after the statement — the gate then dies on a 30 s waitForResponse timeout (order-gate.mjs:724 in run 39996387148).
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
    else if (depth === 0 && c === "a" && /^await\b/.test(text.slice(i, i + 6))) return i; // slice one past `await`: the word "awaited" in prose is not an await
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

// A wait awaited inside its own expression: nothing can trigger the request between registering and awaiting it, so the
// trigger is either still ahead (the await hangs) or already past (the reply was missed).
const INLINE_AWAIT = /\bawait\s+\(*\s*(?:[A-Za-z_$][\w$]*(?:\([^()]*\))?\.)*(historyRead|waitForResponse|waitForRequest)\s*\(/g;

// The user action that fires a request. Used only inside a Promise.all span, to require the wait to be listed first.
const TRIGGER = /\.(?:click|dblclick|fill|press|selectOption|check|uncheck|goto|tap|type|setInputFiles|reload)\s*\(/g;

// Span just past `Promise.all([ ... ])`, or -1. Balances brackets so a nested call/array stays inside the span.
function aggregateSpan(text, from) {
  let depth = 0;
  for (let i = from; i < text.length; i++) {
    const c = text[i];
    if (OPENERS.has(c)) depth++;
    else if (CLOSERS.has(c)) { depth = Math.max(0, depth - 1); if (depth === 0) return i + 1; }
  }
  return -1;
}

// files: [{path, text}]. Returns one human-readable finding per wait that can reject unhandled or race its trigger.
export function offenders(files) {
  const found = [];
  for (const { path, text } of files) {
    const re = new RegExp(WAIT.source, "g");
    const names = new Set();
    let match;
    while ((match = re.exec(text))) {
      const [whole, name] = match;
      names.add(name);
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

    // Ordering rule 1: a wait awaited inside its own expression (await-wait-then-trigger).
    const inline = new RegExp(INLINE_AWAIT.source, "g");
    let im;
    while ((im = inline.exec(text))) {
      const line = text.slice(0, im.index).split("\n").length;
      found.push(`${path}:${line} ${im[1]} is awaited in its own expression; register the wait before the action that triggers the request and await it in Promise.all with that action`);
    }

    // Ordering rule 2: Promise.all([trigger(), wait]) — a reply held by the fixture can only be released after the
    // statement, so the wait must be listed first (Promise.all([wait, trigger()])).
    const all = /Promise\.all\s*\(\s*\[/g;
    let am;
    while ((am = all.exec(text))) {
      const start = text.indexOf("[", am.index);
      const end = aggregateSpan(text, start);
      if (start < 0 || end < 0) continue;
      const inside = text.slice(start, end);
      TRIGGER.lastIndex = 0;
      const t = TRIGGER.exec(inside);
      if (!t) continue;
      let waitAt = -1;
      for (const name of names) {
        const n = new RegExp(`\\b${name}\\b`).exec(inside);
        if (n && (waitAt < 0 || n.index < waitAt)) waitAt = n.index;
      }
      const w = /(historyRead|waitForResponse|waitForRequest)\s*\(/.exec(inside);
      if (w && (waitAt < 0 || w.index < waitAt)) waitAt = w.index;
      if (waitAt >= 0 && t.index < waitAt) {
        const line = text.slice(0, start + t.index).split("\n").length;
        found.push(`${path}:${line} the trigger is listed before the wait inside Promise.all; list the wait first`);
      }
    }
  }
  return found;
}
