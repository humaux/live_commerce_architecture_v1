# Independent follow-up review ledger

These are non-author review results; none substitutes for the root's runtime gates.

| Reviewer | Scope | Finding / disposition | Humaux record |
|---|---|---|---|
| product_contract | Six receipt/UI/test files, author 72f36edb against 40128fe5; checked root fresh-source fixture 0c5f93f3 | No P0/P1. P2: authoritative UNKNOWN same-key POST replays its cached receipt, so it is not a progress poll. | 34d69c85-36c2-49b5-b2d7-c9cd9230aed9 |
| product_contract | Three-file correction 62a77c98 (root 4acbad53) | CLEAR. No progress-POST for authoritative UNKNOWN; new request stays disabled. Transport UNKNOWN preserves its original-key retry. Reload retains journal without a new POST. | 38111522-a424-44f1-bb52-e5d609143e35 |
| orders_visual_final | 0f1ee6eb real screenshots at 390/1586 in all three locales | P1: approximately 100 joined UUIDs push mobile session results thousands of pixels down. No other blocking finding in this scope. | `ads-attribution 0f1ee6eb independent visual QA: linked-draft ID wall P1` |
| orders_visual_final | Final 4acbad53 screenshots and six click ledgers, root 52674e06 fix | RESOLVED / ship. Collapsed count is 103; all IDs retained and exact server-list comparison, 44px target, keyboard close passed. No new blocking visual regression. | `ads-attribution 4acbad53 ID-wall P1 visual rescore resolved` |
| product_contract | Final source-stamped static/PG/browser/sweep/edge evidence and SUMMARY, while G07 runs | No P0/P1 evidence overstatement. Paid-only cohort, unknown values and 100-draft/10k-order cold timing agree. Non-blocking stale wording fixed: focused/browser already pass, only overall G07 sign-off remains pending. | de081b0d-8382-4386-8e0e-fa7df09e02de |
| product_contract | Final G07 ledger, skip list, frozen source and machine-load evidence | CLEAR: 6,725 PASS / 0 FAIL / 13 accepted NOT_RUN and source 4acbad53 match; 131 samples, 1m peak 20.87 / six >=10, 5m max 8.51 match. No continuous-idle overclaim. | 065bcb9c-350b-4ce3-b060-caced5a81e21 |
| product_contract | Post-staging G04 and auxiliary artifact whitespace disclosure | CLEAR: G04 exit 0 supported; optional raw-artifact whitespace check exit 2 is explicitly disclosed, not labelled PASS. Reviewer accidentally invoked that read-only whitespace check during a quoted search; no files or tests changed. | 32d5f1c3-2662-4807-bfa3-8e63949274bf |

Reviewers did not rerun PG or browser. Expanded identifier-state screenshot was
not separately captured; the real browser did expand it, compare the full list,
and close it by keyboard. Native date popovers, physical touch gestures and LIVE
production data were not inspected. Root separately inspected final TW mobile
and English desktop images. No general restyling was performed.

Root runtime evidence: `browser-final/`, `browser-attribution.log` and the final
source-stamped gate ledgers. Earlier SQL non-author counterexamples and their
fixes remain documented in `IMPLEMENTATION.md` and author evidence directories.
