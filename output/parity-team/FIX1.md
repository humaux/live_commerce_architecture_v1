# PARITY-TEAM-FIX1 — BLOCKED, no implementation push

## Current session revalidation

FIX1 revalidation 2026-11-01T07:17:27Z: fetched and merged origin/r3/integration first; already up to date at 4b8c04c7213025c173e260db6f9c81cb5b818c61. PR #476 head remains 89452970349f92219e1b1217868611f1a73057fe. Fresh failed-job reads confirm expected tests-only RED (Gates 41279053547 / browser 124769729983, missing signed 員工名單); receipts 124775330079 and required 124775560803 are downstream. Original implementation card and signed DELIVERY were read at their authoritative read-only paths; staff.go:189 still sends after commit, 0089 explicitly excludes an outbox, and 0090 requires order_id and order-mail kinds. No permitted invitation enqueue interface found. Signed unchanged-role NOT_SENT/disabled Continue still conflicts with mandatory review/result frame coverage. PR comments contain no ruling. BLOCKED by original card STOP boundaries, not another fix attempt. Ownership generation/check, evidence retention and git diff --check exit 0; no generated diff. Existing ignored artifact log/manifest/ledger hashes match the inventory. No source/test edits, commit, push, local tests or server. E3 proves RED only; GREEN NOT_RUN; WAIT_RUN: none. Humaux tools are not exposed in this runtime: search/coord/store/canvas NOT_RUN; older availability statements below are historical and do not describe this runtime.

Date: 2026-11-01. Role: integrator-dispatched implementation lane; no recursive delegation or separate claim. Runtime model/reasoning identifiers UNKNOWN (not exposed).
Branch: `unit/parity-team`; PR #476 → `r3/integration`.
Head: `89452970349f92219e1b1217868611f1a73057fe`.
Current relay revalidation: `2026-11-01T07:14:22Z`. Fetched and merged integration first (exit 0, Already up to date). Fresh PR checks and all three failed-job logs confirm the same exact-head causal RED: 168/223 browser failures, receipts and required downstream. Original implementation card, current staff sender/outbox schema and signed SPEC rows 181–183 were read directly; both STOP constraints below remain, with no PR comment ruling. Humaux tools became available after initial discovery and were used for targeted search, rejected subset (not exhaustive), development rules, coord/canvas reads and handoff. Ownership generation/check, retention and diff check exit 0; no generated diff. No product/test/assertion edit, fix attempt, commit, push, local test or server. E3 RED only; GREEN NOT_RUN; WAIT_RUN: none.
Latest original-evidence revalidation: `2026-11-01T07:11:13Z`. Fetch/merge exit 0, Already up to date. PR comments remain empty; exact-head Gates and Foundation metadata and all three failed-job logs were read again. Failure-log, manifest and click-ledger hashes below match the existing ignored artifact files. Original implementation card, current Staff.Invite/API/migration source and signed DELIVERY rows 181–183 still impose both STOP constraints below. No transport or frame-entry ruling was supplied. Humaux is available. Ownership generation/check, evidence retention and diff check exit 0; no generated diff. No local tests/servers, source/assertion edits, commit or push. WAIT_RUN: none.
Fetched/merged integration: `4b8c04c7213025c173e260db6f9c81cb5b818c61`, exit 0, Already up to date. Entry dirty DELIVERY.md, PR-BODY.md, red.log and evidence/ retained.
Gates 41279053547 tested PR merge SHA `f9f47e0199b6f8f2af7a40ab5e7fb2d7bf2a8d71`; head metadata is the exact head above.

## Every failing job → test → evidence → class

| Failing job | Failing test/assertion | Evidence | Class / disposition |
|---|---|---|---|
| 124769729983, `gate (--browser-admin-parity)` | 168 newly added PARITY-TEAM cases; exact names indexed below. 223 total, 55 pass, 168 fail. | Run 41279053547; `gate-browser-admin-parity`; log and manifest hashes below. Representative `parity-team.mjs:171`: `main.getByRole(button, name=員工名單, exact=true)` absent, timeout 1500ms. Screenshot `parity/1468-74273.png` shows existing inline invite form and role dropdowns. | Product-side missing implementation / expected causal RED. Closest bucket **(b)**, with an explicit distinction: this is a tests-only head, not a regression introduced by product changes. No (a) obsolete tests identified and no (c) base failure proved. Keep assertions; implementation is blocked by the original card constraints below. |
| 124775330079, `receipts` | `Every planned mode has one fresh PASS receipt with tests` | Raw line: `FAIL: --browser-admin-parity: status FAIL`; receipt test_count=223, run=41279053547/1, merge SHA above. Browser log already prints `## matrix-receipts browser-spec`. | Downstream of the product gap, not an independent receipt-marker defect. |
| 124775560803, `required` | required status aggregation | Raw lines: `GATE: failure`, `RECEIPTS: failure`; build success, aggregate skipped (expected false). | Downstream of the same browser job. |

Aggregate job 124775331462 was SKIPPED, not failed. No inbox mode ran in this PR run; known inbox failures are not evidence for class (c) here.
Foundation 41279053499 / 124767027990 is successful on this exact head. Build 124767860982 successful. Neither is product GREEN.
Base run 41277975588 was successful but selected only unit + g01..g10, without browser-admin-parity. It cannot prove browser equivalence or class (c).

### First-failure families in the 164 signed-frame captures

| First assertion failure | Capture count | Class |
|---|---:|---|
| `signed nav: 設定與員工` | 2 | expected RED / missing signed UI; nav authority caveat below |
| `signed members tab replaces legacy always-open sections` | 126 | expected RED / missing signed UI; nav authority caveat below |
| `signed empty: 目前只有你一位員工` | 2 | expected RED / missing signed UI; nav authority caveat below |
| `signed password-off: 暫時無法邀請員工` | 2 | expected RED / missing signed UI; nav authority caveat below |
| `signed non-owner: 只有店主可以管理員工` | 2 | expected RED / missing signed UI; nav authority caveat below |
| `signed load-error: 暫時無法讀取員工` | 2 | expected RED / missing signed UI; nav authority caveat below |
| `signed invitation heading` | 28 | expected RED / missing signed UI; nav authority caveat below |

Four additional failures are the zh-CN/en main-screen checks at both widths. Manifest: 164/164 `matched=false`; click ledger: zero clicks recorded. This is failed acceptance evidence, not evidence that 164 states were reached. No screenshot is counted as a passed control/persistence check.

Nav caveat: the new nav cases assert module-frame copy 設定與員工, while the card explicitly permits the owner-approved shared shell. Do not silently change that shell to satisfy a module snapshot. These two assertions need reconciliation against the shell authority during the implementation batch; no assertion or fixture was changed in this blocked session.

## Root blocking constraints, re-read from source

1. **No invitation-capable existing outbox was found.** Original implementation card `output/integrator/tools/cards/cxf-parity-team-impl.md` requires: “Invitation email goes only through the existing mail outbox, MOCK in tests.” At current source, `internal/identity/staff.go:189` calls `mailer.Send` after the invitation transaction; `cmd/api/identity.go:316` passes the SMTP adapter. `migrations/0089_staff_team.sql:484` explicitly states “not a mail outbox (one send after commit, never retried, I06)” and stores only token hashes. `migrations/0090_buyer_comms.sql:36-38` requires `order_id` and a closed list of order-mail kinds. Scoped identity/mail/notify/jobqueue/migration search found no staff invitation enqueue/claim interface. Integrator must identify the intended existing interface or amend transport scope and assign migration/token-storage policy. A new queue/contract/other-backend-module change is expressly a STOP under FIX1, not covered by the in-admin scope pre-approval. No migration, contract, token storage, mail transport or pause guard was changed.
2. **Unreachable no-op-role frames remain unresolved.** Signed ux-team DELIVERY SPEC rows 181–183 require unchanged-role NOT_SENT; the editor's Continue is disabled. Required review/result frame IDs are `1468:78293`, `1468:78333`, `1468:78348`, `1468:78392`. `tests/admin/parity-team.mjs:184-191` preserves disabled/no-write behavior, with no real-click path into those states. No signed entry-path or scope amendment is in the PR comments/current source. Integrator must resolve that path or correct signed scope. No forced click, invented transition, skip, exemption or claimed coverage.

These are source/authority constraints (E1), not failed fix attempts. No implementation attempt or additional push was made by FIX1. The two-attempt ceiling was not bypassed by changing assertions. This report replaces repetitive relay checks with an explicit STOP; do not re-dispatch unchanged instructions as a new fix approach.

## Evidence pointers (raw downloads stay ignored)

- Run: 41279053547; job: 124769729983; artifact: `gate-browser-admin-parity`, ID 13072894279, archive digest `sha256:126260df4cd64fe05ff0be13935f217dc365540f15b6992e3ccd121df4c77201` (GitHub artifact metadata).
- `--browser-admin-parity.log` SHA-256 `df22864d3cc2eb82805a6d4a085e735c950fa3aaa1036e5b165211084cbb23c4`.
- `team-manifest.json` SHA-256 `3d02aa71c2e6fca1f1391e8a811e516cb81139442b75d0319e9b229dfd768ac4`.
- `team-click-ledger.json` SHA-256 `c6434fcf33114b2428293165c706676f1871a9f5814cff1f00f45895b99f5f3f`.
- Representative inspected screenshot `parity/1468-74273.png` SHA-256 `a89ea6323619c75523ce991d8bed3ea6ce46a707c9b0145e4c61180866e3addc`.
- Authoritative copy `output/parity-team/FRAME-COPY.md`: `fd1ff3d7c2f1eceb2a6aec157886925fd5bf31e2ea307a0e79f70bc97fee55c0`.
- Signed ux-team DELIVERY: `6a7c24e81a263a0e04b1731701ec691f5b865f94ad68874aa0b726a0b7490b75`; read only at original handoff path.

## Actions and handoff

- Local tests/servers: NOT_RUN by owner requirement. Only evidence reads and explicit browser-ownership generation/check performed; generation/check both exit 0 and generated file has no new diff.
- No product/test/selector/source change, new commit, push, merge of PR, auto-merge or posted status. No CI started. Existing PR remains tests-only RED.
- FIX1 writes only own `output/parity-team/FIX1.md`, brief DELIVERY/red.log/PR-body notes. Files outside card write paths: none this session. No dep-map change needed because no source dependency changed.
- Downloaded CI artifact is in ignored `output/parity-team/ci-artifacts/` and is NOT staged or committed; retained as failed evidence. No started process/server/fixture/port remains.
- E3: exact-head CI proves expected RED only. E1: source constraints + static ownership consistency. Author PASS would be E0. GREEN, implementation, successful real-click/persistence acceptance, pause-lift/outbox, independent review, SANDBOX/LIVE: NOT_RUN.
- WAIT_RUN: none (no push). Integrator owns resolution of the transport/scope and four-frame entry path before resuming implementation.

## Exact failing-test index (derived from CI failure summary)

| # | Test | Evidence | Class |
|---:|---|---|---|
| 1 | PARITY-TEAM signed 1466:74190 nav | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 2 | PARITY-TEAM signed 1468:74273 list | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 3 | PARITY-TEAM signed 1468:74492 pending | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 4 | PARITY-TEAM signed 1468:74559 empty | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 5 | PARITY-TEAM signed 1468:74622 invite-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 6 | PARITY-TEAM signed 1468:74741 invite-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 7 | PARITY-TEAM signed 1468:74860 invite-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 8 | PARITY-TEAM signed 1468:74979 invite-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 9 | PARITY-TEAM signed 1468:75098 invite-viewer | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 10 | PARITY-TEAM signed 1468:75217 invite-sent | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 11 | PARITY-TEAM signed 1468:75278 invalid-email | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 12 | PARITY-TEAM signed 1468:75349 already-member | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 13 | PARITY-TEAM signed 1468:75412 invite-limit | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 14 | PARITY-TEAM signed 1468:75475 owner-disabled | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 15 | PARITY-TEAM signed 1468:75538 reauth | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 16 | PARITY-TEAM signed 1468:75601 mail-failed | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 17 | PARITY-TEAM signed 1468:75668 password-off | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 18 | PARITY-TEAM signed 1468:75731 non-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 19 | PARITY-TEAM signed 1468:75792 role-edit | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 20 | PARITY-TEAM signed 1468:75905 role-confirm | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 21 | PARITY-TEAM signed 1468:75960 role-done | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 22 | PARITY-TEAM signed 1468:76023 list-updated | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 23 | PARITY-TEAM signed 1468:76100 remove-confirm | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 24 | PARITY-TEAM signed 1468:76155 remove-done | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 25 | PARITY-TEAM signed 1468:76218 revoke-confirm | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 26 | PARITY-TEAM signed 1468:76275 revoke-done | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 27 | PARITY-TEAM signed 1468:76338 pending-empty | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 28 | PARITY-TEAM signed 1468:76405 invite-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 29 | PARITY-TEAM signed 1468:76476 role-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 30 | PARITY-TEAM signed 1468:76547 remove-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 31 | PARITY-TEAM signed 1468:76618 revoke-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 32 | PARITY-TEAM signed 1468:76689 checking-unresolved | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 33 | PARITY-TEAM signed 1468:76754 load-error | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 34 | PARITY-TEAM signed 1468:76817 last-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 35 | PARITY-TEAM signed 1468:76880 link | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 36 | PARITY-TEAM signed 1468:76904 signin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 37 | PARITY-TEAM signed 1468:76946 signup | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 38 | PARITY-TEAM signed 1468:76990 verify-signin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 39 | PARITY-TEAM signed 1468:77022 verify-signup | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 40 | PARITY-TEAM signed 1468:77054 accept | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 41 | PARITY-TEAM signed 1468:77078 accept-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 42 | PARITY-TEAM signed 1468:77116 joined | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 43 | PARITY-TEAM signed 1468:77152 expired | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 44 | PARITY-TEAM signed 1468:77188 revoked | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 45 | PARITY-TEAM signed 1468:77224 used | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 46 | PARITY-TEAM signed 1468:77260 wrong-account | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 47 | PARITY-TEAM signed 1468:77296 invitee-member | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 48 | PARITY-TEAM signed 1468:77332 invitee-off | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 49 | PARITY-TEAM signed 1468:77368 invite-sent-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 50 | PARITY-TEAM signed 1468:77429 pending-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 51 | PARITY-TEAM signed 1468:77496 invite-sent-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 52 | PARITY-TEAM signed 1468:77557 pending-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 53 | PARITY-TEAM signed 1468:77624 invite-sent-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 54 | PARITY-TEAM signed 1468:77685 pending-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 55 | PARITY-TEAM signed 1468:77752 invite-sent-viewer | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 56 | PARITY-TEAM signed 1468:77813 pending-viewer | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 57 | PARITY-TEAM signed 1468:77880 role-edit-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 58 | PARITY-TEAM signed 1468:77993 role-confirm-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 59 | PARITY-TEAM signed 1468:78048 role-done-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 60 | PARITY-TEAM signed 1468:78111 list-updated-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 61 | PARITY-TEAM signed 1468:78180 role-edit-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 62 | PARITY-TEAM signed 1468:78293 role-confirm-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 63 | PARITY-TEAM signed 1468:78348 role-done-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 64 | PARITY-TEAM signed 1468:78411 list-updated-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 65 | PARITY-TEAM signed 1468:78488 role-edit-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 66 | PARITY-TEAM signed 1468:78601 role-confirm-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 67 | PARITY-TEAM signed 1468:78656 role-done-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 68 | PARITY-TEAM signed 1468:78719 list-updated-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 69 | PARITY-TEAM signed 1468:78796 role-edit-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 70 | PARITY-TEAM signed 1468:78909 role-confirm-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 71 | PARITY-TEAM signed 1468:78964 role-done-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 72 | PARITY-TEAM signed 1468:79027 list-updated-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 73 | PARITY-TEAM signed 1468:79104 revoke-confirm-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 74 | PARITY-TEAM signed 1468:79161 revoke-confirm-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 75 | PARITY-TEAM signed 1468:79218 revoke-confirm-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 76 | PARITY-TEAM signed 1468:79275 revoke-confirm-viewer | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 77 | PARITY-TEAM signed 1468:80786 role-unresolved | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 78 | PARITY-TEAM signed 1468:80851 remove-unresolved | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 79 | PARITY-TEAM signed 1468:80916 revoke-unresolved | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 80 | PARITY-TEAM signed 1468:86250 list-owner-no-email | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 81 | PARITY-TEAM signed 1468:96313 reauth-signin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 82 | PARITY-TEAM signed 1468:96378 reauth-verify | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 83 | PARITY-TEAM signed 1468:74244 nav | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 84 | PARITY-TEAM signed 1468:74324 list | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 85 | PARITY-TEAM signed 1468:74538 pending | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 86 | PARITY-TEAM signed 1468:74603 empty | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 87 | PARITY-TEAM signed 1468:74694 invite-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 88 | PARITY-TEAM signed 1468:74813 invite-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 89 | PARITY-TEAM signed 1468:74932 invite-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 90 | PARITY-TEAM signed 1468:75051 invite-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 91 | PARITY-TEAM signed 1468:75170 invite-viewer | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 92 | PARITY-TEAM signed 1468:75260 invite-sent | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 93 | PARITY-TEAM signed 1468:75326 invalid-email | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 94 | PARITY-TEAM signed 1468:75393 already-member | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 95 | PARITY-TEAM signed 1468:75456 invite-limit | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 96 | PARITY-TEAM signed 1468:75519 owner-disabled | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 97 | PARITY-TEAM signed 1468:75582 reauth | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 98 | PARITY-TEAM signed 1468:75647 mail-failed | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 99 | PARITY-TEAM signed 1468:75712 password-off | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 100 | PARITY-TEAM signed 1468:75774 non-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 101 | PARITY-TEAM signed 1468:75861 role-edit | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 102 | PARITY-TEAM signed 1468:75945 role-confirm | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 103 | PARITY-TEAM signed 1468:76004 role-done | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 104 | PARITY-TEAM signed 1468:76074 list-updated | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 105 | PARITY-TEAM signed 1468:76140 remove-confirm | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 106 | PARITY-TEAM signed 1468:76199 remove-done | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 107 | PARITY-TEAM signed 1468:76259 revoke-confirm | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 108 | PARITY-TEAM signed 1468:76319 revoke-done | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 109 | PARITY-TEAM signed 1468:76384 pending-empty | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 110 | PARITY-TEAM signed 1468:76453 invite-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 111 | PARITY-TEAM signed 1468:76524 role-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 112 | PARITY-TEAM signed 1468:76595 remove-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 113 | PARITY-TEAM signed 1468:76666 revoke-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 114 | PARITY-TEAM signed 1468:76734 checking-unresolved | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 115 | PARITY-TEAM signed 1468:76798 load-error | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 116 | PARITY-TEAM signed 1468:76861 last-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 117 | PARITY-TEAM signed 1468:76892 link | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 118 | PARITY-TEAM signed 1468:76925 signin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 119 | PARITY-TEAM signed 1468:76968 signup | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 120 | PARITY-TEAM signed 1468:77006 verify-signin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 121 | PARITY-TEAM signed 1468:77038 verify-signup | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 122 | PARITY-TEAM signed 1468:77066 accept | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 123 | PARITY-TEAM signed 1468:77097 accept-checking | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 124 | PARITY-TEAM signed 1468:77134 joined | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 125 | PARITY-TEAM signed 1468:77170 expired | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 126 | PARITY-TEAM signed 1468:77206 revoked | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 127 | PARITY-TEAM signed 1468:77242 used | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 128 | PARITY-TEAM signed 1468:77278 wrong-account | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 129 | PARITY-TEAM signed 1468:77314 invitee-member | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 130 | PARITY-TEAM signed 1468:77350 invitee-off | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 131 | PARITY-TEAM signed 1468:77411 invite-sent-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 132 | PARITY-TEAM signed 1468:77475 pending-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 133 | PARITY-TEAM signed 1468:77539 invite-sent-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 134 | PARITY-TEAM signed 1468:77603 pending-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 135 | PARITY-TEAM signed 1468:77667 invite-sent-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 136 | PARITY-TEAM signed 1468:77731 pending-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 137 | PARITY-TEAM signed 1468:77795 invite-sent-viewer | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 138 | PARITY-TEAM signed 1468:77859 pending-viewer | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 139 | PARITY-TEAM signed 1468:77949 role-edit-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 140 | PARITY-TEAM signed 1468:78033 role-confirm-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 141 | PARITY-TEAM signed 1468:78092 role-done-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 142 | PARITY-TEAM signed 1468:78158 list-updated-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 143 | PARITY-TEAM signed 1468:78249 role-edit-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 144 | PARITY-TEAM signed 1468:78333 role-confirm-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 145 | PARITY-TEAM signed 1468:78392 role-done-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 146 | PARITY-TEAM signed 1468:78462 list-updated-admin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 147 | PARITY-TEAM signed 1468:78557 role-edit-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 148 | PARITY-TEAM signed 1468:78641 role-confirm-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 149 | PARITY-TEAM signed 1468:78700 role-done-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 150 | PARITY-TEAM signed 1468:78770 list-updated-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 151 | PARITY-TEAM signed 1468:78865 role-edit-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 152 | PARITY-TEAM signed 1468:78949 role-confirm-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 153 | PARITY-TEAM signed 1468:79008 role-done-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 154 | PARITY-TEAM signed 1468:79078 list-updated-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 155 | PARITY-TEAM signed 1468:79145 revoke-confirm-owner | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 156 | PARITY-TEAM signed 1468:79202 revoke-confirm-live | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 157 | PARITY-TEAM signed 1468:79259 revoke-confirm-fulfilment | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 158 | PARITY-TEAM signed 1468:79316 revoke-confirm-viewer | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 159 | PARITY-TEAM signed 1468:80831 role-unresolved | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 160 | PARITY-TEAM signed 1468:80896 remove-unresolved | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 161 | PARITY-TEAM signed 1468:80961 revoke-unresolved | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 162 | PARITY-TEAM signed 1468:86301 list-owner-no-email | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 163 | PARITY-TEAM signed 1468:96358 reauth-signin | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 164 | PARITY-TEAM signed 1468:96420 reauth-verify | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 165 | PARITY-TEAM main screen zh-CN desktop | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 166 | PARITY-TEAM main screen zh-CN mobile | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 167 | PARITY-TEAM main screen en desktop | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
| 168 | PARITY-TEAM main screen en mobile | 41279053547 / 124769729983 | expected RED, product-gap bucket (b) |
