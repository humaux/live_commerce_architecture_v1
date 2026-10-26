// Purpose: #179 lexical ratchet for browser gate scripts — a waitForResponse/waitForRequest (or the historyRead wrapper)
//   assigned to a variable must be awaited before the next top-level await. Otherwise an intervening assertion throws, the
//   page closes, the stray wait rejects unhandled and node exits with "Target page ... has been closed", hiding the real
//   first failure (tests/storefront/order-gate.mjs:738 before this fix; see issue #179).
//   Round 2 pins ORDERING for waits CREATED in the aggregate: a wait awaited inline (`const r = await
//   page.waitForResponse(...)`) can only miss the reply or hang, and `Promise.all([trigger(), p.waitForResponse(...)])`
//   registers its listener only after the action dispatches. A wait assigned before the aggregate is already listening,
//   so its array position is lexical only: a pre-registered `pending` may sit after the trigger (#181 P2).
//   Round 3 pins the hold itself: arm(..., {after:true}) withholds the reply until release.resolve(), so a wait declared
//   with that hook must carry an actual `<hook>.release.resolve(` in the entered branch of the same awaited aggregate,
//   after `<hook>.entered` and before the aggregate is awaited. Merely naming `<hook>.entered` (with the release after
//   the statement) still deadlocks; a release driven through a helper variable is not recognised either, so keep the
//   release branch inline in the aggregate.
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

// The source line holding `index`. Gate scripts declare a held hook and its wait on one line
// (`const held=arm(...),wait=historyRead(p);`); a declaration wrapped across lines is outside this lexical ratchet.
function declarationLine(text, index) {
  const start = text.lastIndexOf("\n", index - 1) + 1;
  const end = text.indexOf("\n", index);
  return { start, end: end < 0 ? text.length : end };
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

// Top-level comma-separated elements of an array body (between `[` and `]`), as [start, end) offsets into text.
function topLevelElements(text, from, to) {
  const elements = [];
  let depth = 0;
  let start = from;
  for (let i = from; i < to; i++) {
    const c = text[i];
    if (OPENERS.has(c)) depth++;
    else if (CLOSERS.has(c)) depth = Math.max(0, depth - 1);
    else if (depth === 0 && c === ",") { elements.push([start, i]); start = i + 1; }
  }
  elements.push([start, to]);
  return elements;
}

// True when the awaited aggregate has a branch that names `<hook>.entered` and then calls `<hook>.release.resolve(`:
// that is the only shape whose release runs after the request is actually held, inside the awaited statement.
function releasedInEnteredBranch(statement, hook) {
  const entered = new RegExp(`\\b${hook}\\.entered\\b`).exec(statement);
  if (!entered) return false;
  const all = /Promise\.(?:all|allSettled|race)\s*\(\s*\[/.exec(statement);
  if (!all) return false;
  const open = statement.indexOf("[", all.index);
  const end = aggregateSpan(statement, open);
  if (open < 0 || end < 0) return false;
  const release = new RegExp(`\\b${hook}\\.release\\.resolve\\s*\\(`);
  for (const [from, to] of topLevelElements(statement, open + 1, end - 1)) {
    if (entered.index < from || entered.index >= to) continue;
    const call = release.exec(statement.slice(from, to));
    return Boolean(call && from + call.index > entered.index);
  }
  return false;
}

// files: [{path, text}]. Returns one human-readable finding per wait that can reject unhandled or race its trigger.
export function offenders(files) {
  const found = [];
  for (const { path, text } of files) {
    const re = new RegExp(WAIT.source, "g");
    const waits = [];
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
      waits.push({ name, line, index: match.index, statement });
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

    // Ordering rule 2: Promise.all([trigger(), <wait creation>]) — a wait CREATED inside the array after the trigger
    // registers its listener only after the action dispatches. A wait assigned before the aggregate is already
    // listening; its position in the array is lexical only, so a pre-registered `pending` may sit after the trigger
    // (#181 P2). Held replies remain covered by rule 3 regardless of array order.
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
      const created = new RegExp(/(?:historyRead|waitForResponse|waitForRequest)\s*\(/g);
      let cm;
      let afterTrigger = false;
      while ((cm = created.exec(inside))) if (cm.index > t.index) { afterTrigger = true; break; }
      if (!afterTrigger) continue;
      const line = text.slice(0, start + t.index).split("\n").length;
      found.push(`${path}:${line} the trigger is listed before an inline wait inside Promise.all; register the wait before the action and list it first`);
    }

    // Ordering rule 3: arm(..., {after: true}) withholds the reply until hook.release resolves. A wait declared in the
    // same statement as that hook must drive the hold from an actual release in the entered branch of the same awaited
    // aggregate; naming hook.entered alone does not release, and a release after the statement is too late (#181 P2).
    const heldArm = /([A-Za-z_$][\w$]*)\s*=\s*arm\s*\(/g;
    let hm;
    while ((hm = heldArm.exec(text))) {
      const holdEnd = callEnd(text, hm.index + hm[0].length - 1);
      if (holdEnd < 0 || !/\bafter\s*:\s*true\b/.test(text.slice(hm.index, holdEnd))) continue;
      const decl = declarationLine(text, hm.index);
      for (const wait of waits) {
        if (!wait.statement || wait.index <= decl.start || wait.index >= decl.end) continue;
        if (releasedInEnteredBranch(wait.statement, hm[1])) continue;
        found.push(`${path}:${wait.line} held ${wait.name} is awaited without ${hm[1]}.release.resolve() in its entered branch; arm(..., {after: true}) withholds the reply until release, so this await times out`);
      }
    }
  }
  return found;
}
