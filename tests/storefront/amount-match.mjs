// Purpose: whole-token money matchers for browser gates: server minor units -> the TWD digits the pages show, and a RegExp that finds that amount as its own token.
// Depends on: nothing (pure functions).
// Used by: tests/storefront/promotions-gate.mjs (amt, withoutFixtureIds for negative checks); self-checked by tests/storefront/amount-match.test.mjs (scripts/dev/test-node.sh).
// Invariant: an amount never matches inside another number ("960" in "1,960" / "9600") or inside a fixture id built from random hex ("960" in
// "BRP-40faa960dbfa", CI run 38727832594): before the number ASCII letters/digits/","/"." are refused, so a currency prefix ("NT$960") still matches.
// After the number only digits/"," are refused, not letters: Playwright's text joins adjacent elements ("Order totalNT$1,060Quote valid…",
// "NT$200NT$300直播價"), so a label can follow an amount directly (PR #72 first CI run: refusing letters there failed every en cell).
// A hex tag that starts with the digits ("BRP-960ab…") still matches after "-": negative checks strip fixture ids first (withoutFixtureIds).
export const money = minor => (minor / 100).toLocaleString("en-US");
export const amountMatcher = shown => new RegExp(`(?<![0-9A-Za-z,.])${shown.replace(/,/g, "[,]")}(\\.00)?(?![0-9,])`);
export const amt = minor => amountMatcher(money(minor));
// Negative checks scan the whole quotation, which also shows the gate's own random fixture ids: the SKU code "BRP-<12 hex>"
// (tests/foundation/browser_promotions_test.go, t04Tag) and UUIDs. Remove exactly those formats first (same approach as
// tests/e2e/live-tools.spec.ts withoutFixtureIds); no rendered amount can match them, and amt() itself is unchanged.
export const withoutFixtureIds = text =>
  text.replace(/[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}/gi, " ").replace(/\bBRP-[0-9a-f]{12}\b/gi, " ");
