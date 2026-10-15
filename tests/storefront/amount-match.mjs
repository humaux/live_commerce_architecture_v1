// Purpose: whole-token money matchers for browser gates: server minor units -> the TWD digits the pages show, and a RegExp that finds that amount as its own token.
// Depends on: nothing (pure functions).
// Used by: tests/storefront/promotions-gate.mjs; self-checked by tests/storefront/amount-match.test.mjs (scripts/dev/test-node.sh).
// Invariant: an amount never matches inside another number ("960" in "1,960" / "9600") or inside a fixture id built from random hex ("960" in
// "BRP-40faa960dbfa", CI run 38727832594), so ASCII letters/digits/","/"." are refused on both sides; a currency prefix ("NT$960") still matches.
export const money = minor => (minor / 100).toLocaleString("en-US");
export const amountMatcher = shown => new RegExp(`(?<![0-9A-Za-z,.])${shown.replace(/,/g, "[,]")}(\\.00)?(?![0-9A-Za-z,])`);
export const amt = minor => amountMatcher(money(minor));
