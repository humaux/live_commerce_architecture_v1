# PARITY-ENTRY-FIX2 relay 4 report

Remote head 3be83d29 of PR #369 showed CONFLICT with no checks (`gh pr checks 369`: "no checks reported"; mergeable=CONFLICTING). No CI jobs ran on that head, so there are no new failing logs. Run 41170112825 (older head aaef95b97) is the stale run listed in FAILS.

| Failing job/test | Class | What changed (file:line) |
|---|---|---|
| 124446852658 `TestBrowserInboxUIRealChain` / `--browser-inbox` (run 41170112825, stale head) | b | Already fixed in earlier relays (a74dfa7c4; see output/PARITY-ENTRY/FIX2.md). Run 41173868839 shows bundle:06 passing. No new change. |
| 124446852816 `TestBrowserPasswordAuth` / `--browser-password-auth` (stale head) | a + b | Already fixed in earlier relays: `tests/admin/parity-entry-dashboard.ts` (cache-control headers, `.000000` timestamps), `apps/admin/components/PasswordAuth.tsx` (hint moved out of label), `tests/admin/password-auth.spec.ts` `visible(email)` rewritten to the signed sent/code frames. No new change. |
| PR #369 mergeable=CONFLICTING on 3be83d29 | merge | Local HEAD already contains origin/r3/integration (merge 2c9bbc753; `git merge origin/r3/integration` = "Already up to date"; `git merge-base --is-ancestor` true). No conflict markers in tracked non-output files. |
| foundation browser ownership drift | - | `node scripts/dev/derive-browser-ownership.mjs` regenerated; `--check` rc=0; no tracked diff. |

Not run (owner rule): tests, builds, browsers. CI on the pushed head is the only verification (E1 here).
Integrator action: push the local branch (3be83d29..HEAD), then WAIT_RUN on the new head.

STATUS: DONE
