// Purpose: whole-token money matchers for browser gates: server minor units -> the TWD digits the pages show, and a RegExp that finds that amount as its own token.
// Depends on: nothing (pure functions).
// Used by: tests/storefront/promotions-gate.mjs; self-checked by tests/storefront/amount-match.test.mjs (scripts/dev/test-node.sh).
// Invariant: an amount never matches inside another number ("960" in "1,960" / "9600") or inside a fixture id built from random hex ("960" in
// "BRP-40faa960dbfa", CI run 38727832594): before the number ASCII letters/digits/","/"." are refused, so a currency prefix ("NT$960") still matches.
// After the number only digits/"," are refused, not letters: Playwright's text joins adjacent elements ("Order totalNT$1,060Quote valid…",
// "NT$200NT$300直播價"), so a label can follow an amount directly (PR #72 first CI run: refusing letters there failed every en cell).
// ponytail: a hex tag that starts with the digits ("BRP-960ab…", 1/4096 per tag) still matches after "-"; strip fixture ids first if that ever flakes.
export const money = minor => (minor / 100).toLocaleString("en-US");
export const amountMatcher = shown => new RegExp(`(?<![0-9A-Za-z,.])${shown.replace(/,/g, "[,]")}(\\.00)?(?![0-9,])`);
export const amt = minor => amountMatcher(money(minor));
