<!-- Purpose: hand off W5-U1 after the merged W6 base, with current local acceptance and CI limits.
Depends on: r3/integration 97e34a4c, migration-import-v1 APIs 0152/0156, signed MOCK/REAL_PG browser evidence.
Used by: integrator K3 pre-review, push, PR creation and GitHub acceptance. -->
# W5-U1 import wizard — post-W6 delivery

- Branch: `unit/w5-u1-import-wizard-ui`; merged trunk: `97e34a4c7646de9d0753fb83ed88d56c1b43f01c` (PR #3 merged).
- Tested source: `224df03a102f3a90815b98132f08c07014fc4f97`. Final documentation commit and 45-file SHA256 binding are in the main checkout's `output/w5-u1-import-wizard-ui/after-pr3-merge/final-receipt.json`.
- Owner: Codex root; read-only explorer/high review, no overlapping writes or recursive delegation. Exact deployed model identifier unavailable.
- Evidence: E3 in the tested local environment, BROWSER signed MOCK IdP + REAL_PG synthetic data. Provider/production acceptance is not claimed.

## Changes and merge

Merged the approved W6 customer/detail, notes privacy/own-author controls, reports and tests. The 18 pure-W6 conflicts use the incoming approved version; CustomerDetail/Customers/copy retain both W6 behavior and the W5 history/import additions. Gate and runner lists are unions: both parents' 80 modes are retained in the resulting 81. No W6 duplicate-fix reversion, backend contract, schema, lockfile or migration changes.

The earlier K3 fixes remain: every import response is private/no-store, coded refusals stay actionable, and a replay-confirmed receipt mismatch terminates UNKNOWN. Raw CSV stays in memory; previews/failure files contain safe row verdicts; consent remains unknown; history is read-only and excluded from revenue.

The first real local acceptance runs found one product defect: oversized files used the generic unreadable-header message. ImportWizard now selects the existing localized 2 MiB guidance; the size bound is unchanged. Test integration corrections follow actual contracts: data rows count after the header, denied readers reach the shell refusal, and erased imported-only customers disappear from the customer projection (POST 200, refreshed GET 404, exact DB deltas and tombstone refusal).

W6 regression locators now select the shared tags/notes recovery status, excluding the new archive Refresh. Its one-shot read-fault test waits for actual completed reads before arming the fault and explicitly verifies the refreshed GET 503; all original failure/retry/CAS/draft/privacy assertions remain.

## Local commands and actual results

Evidence root (main checkout): `output/w5-u1-import-wizard-ui/after-pr3-merge/`. Every command has a raw log and status JSON; browser status includes the exact tested SHA. All PG/browser modes ran serially.

| Command | Exit | Evidence / result |
|---|---:|---|
| `node --test --experimental-strip-types tests/admin/shell-registry.test.ts` | 0 | registry.log: 11 pass |
| `bash scripts/dev/test-node.sh` | 0 | customer-ready-node.log: 936 pass, 0 fail |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | customer-fix-types.log |
| Strict tsc of import-wizard/customer-tags/reports specs | 0 | customer-ready-spec-types.status.json has exact arguments |
| `bash scripts/dev/check-gates.sh` | 0 | customer-ready-gates.log: 81 modes |
| `bash scripts/dev/test-local.sh --browser-migration-import` (fault unset) | 0 | migration-import-final-green: 15/15 + Go harness pass at 224df03a |
| `bash scripts/dev/test-local.sh --browser-customers-billing` | 0 | customers-billing-attempt4: CB11 admin/buyer + W6 9/9, both Go harnesses pass at 224df03a |
| `LC_MIUI_CALIBRATION=drop-customer-commit bash scripts/dev/test-local.sh --browser-migration-import` | 1 expected | calibration-drop-commit: MIUI-RED-COMMIT-RECEIPT |
| `LC_MIUI_CALIBRATION=truncate-preview bash scripts/dev/test-local.sh --browser-migration-import` | 1 expected | calibration-truncate-preview: MIUI-RED-PREVIEW-DATA |
| `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` | 0 | pr-modes.json: 51 planned modes |
| `git diff --check 97e34a4c -- ':!output'` | 0 | source-diff-check.status.json |

Both calibrations fail after real signed readiness for the named injected fault; the final fault-free 15/15 run follows them on unchanged source. Production Next builds ran through the browser runners. Their Go commands include `-race -tags browser` against real PostgreSQL; no separate backend source change requires a new focused Go regex.

## Red → green record

- W6 scope Node loader lacked the new historical child: node-integration-red exit1; actual child-boundary registration preserved scope-loss assertions, scope-green 5 pass/0 and full Node green.
- Reader readiness: reader-ready-red 1 pass/1 fail exit1 → reader-ready-green 2 pass exit0.
- Oversize guidance: browser attempt2 RED and size-red 5 pass/1 fail exit1 → actual header/form test size-green 6 pass exit0 in all three locales → full import browser green.
- Import browser attempts1/3 exit1 retained in browser-red-1/browser-red-3: row/reader and imported-only erasure oracle mismatches. Updated assertions add precise network/DB facts; full15 green.
- Customer regression REDs are retained in customer-red, customer-red-2 and customer-red-3. The first locator repair was too narrow and timed out; the second uses the actual shared status. A separate earlier read-fault race was fixed with completed-read barriers. Final CB11 + W6 nine pass. No failed attempt is counted as green.

The protocol's one-argument planner invocation returned usage exit1; this repository requires explicit base and head. The corrected command above exited0. This setup error is not behavioral red evidence.

## CI gates / NOT_RUN

Ready for integrator K3 pre-review, then push and open the W5 PR. No push/PR/force-push/deploy was performed. Own origin/unit/w5-u1-import-wizard-ui was absent at each pre-commit fetch; absence is recorded, not represented as a merge.

Run normal `--browser-migration-import`, the two named calibration environments, and `--browser-customers-billing` on GitHub. The remaining planned regression set is recorded verbatim in pr-modes.json; full click-sweep shards/aggregate, visual-lint, admin-shell and the other planner modes are NOT_RUN locally in this batch and remain CI acceptance. Full foundation, optional R04 binary, provider SANDBOX/LIVE, production, money and real PII: NOT_RUN.

Current local gates are complete; independent cross-family K3 and full GitHub regression acceptance remain outstanding. Existing w5-w6 follow-up automation is already PAUSED. All owned processes finish before final handoff; shared PostgreSQL resources are left to the runner's lifecycle.
