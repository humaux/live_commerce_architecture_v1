# PR394 direct definer boundary matrices — E1 static candidate

- task_id: eeabdfd4-7437-4e14-822c-34723b237412; child canvas: eeabdfd4-sub-1.
- base_commit: d70573e9a01d4357acb649896dbc8c69c3748310; branch: codex/pr394-offline-diagnostic.
- worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr394-offline-diagnostic.
- role: delegated test writer; requested Sol/high; actual model/provider/reasoning telemetry: UNKNOWN.
- owned paths: tests/foundation/manual_discount_test.go; tests/foundation/manual_discount_mutation.py; output/feat-manual-discount/DIRECT-BOUNDARY-MATRIX.md.
- authority: root frozen tests-only task and schema/ACL clarification; current cart-quote-v1 amendment and storefront-v2 G3; 0220 at base SHA; independent adjudication review394-T-adjudication-1343.md SHA256 49f641176e4c16effb58df645a0d9718b21f2506f40f52928c4acff92ad2ce08.
- related DAG/invariants read: T04/T12/T21; I01/I02/I08/I18/I21; architecture 0/1/2/26/27 and AGENTS.md.
- evidence classification: E1 STATIC (gofmt, AST, source preservation, vet, test compilation). REAL_PG/MOCK/SANDBOX/LIVE runtime and mutation RED/GREEN: NOT_RUN. No product exploit or whole-PR acceptance claim.

## Implementation

`mdDBBoundaryContract/bind_authority` adds ten leaves: one direct matching both-permission positive; three missing-permission negatives (catalog, inventory, neither); independent tenant/store/principal GUC mismatches; same-principal nonempty buyer GUC; actual buyer and checkout login denials. Each negative has a fresh valid positive from the same real CreateManualQuote/seed constructor. Every authorization is seeded under its own caller principal; each full-permission positive and its quote/auth have the same principal. Missing-permission positives use a separate full-permission principal, without granting/revoking permissions on the restricted caller. Restricted target fixtures independently read back the exact tenant/store/owner/principal/hash/name/amount/NULL tuple and real quote adjustment before the direct attempt. No existing grants/fixtures are rewritten.

All merchant attempts enter through store:read, have callback and target-call witnesses, and capture the SQL target error separately from WithScope's result. No RequirePermission can provide their expected PT403. Rejections must retain the entire to_jsonb authorization byte sequence, NULL binding, and complete frozen quote bytes. A positive bind must change only the quote_id field of that same complete tuple and retain quote bytes.

`read_adjustments_authority` adds ten leaves: exact four-field named adjustment and exact [] controls; missing orders:read via a store:read-only caller; independent tenant/store/principal GUC mismatches; actual same-tenant foreign-store and cross-tenant checkout order IDs with correct caller GUCs (PT404 and no data); actual buyer/checkout access denials. Manual orders use the existing merchant HTTP pipeline. Foreign orders use existing moBeginInOtherStore (real pricing/buyer/checkout fixture API with enabled constraints), with independent tenant/store/order readback. Every merchant negative has a fresh real manual-order exact-JSON positive. No mixed-buyer reader rejection contract is introduced.

Actual roles require current_user=session_user, intended role USAGE, no MEMBER path to commerce_auth/commerce_runtime or super/bypass/role-admin/database-creator/replication authority. Function OIDs are resolved independently using the owner pool; actual role transactions assert has_function_privilege(current_user, oid, EXECUTE)=false. Binder additionally requires fulfillment schema USAGE=true. Per root clarification, reader requires the existing identity schema USAGE=false and labels direct 42501/no-data as **schema-or-function access denied**; it does not claim that the function body was entered. OID privilege readback independently proves the function ACL, while merchant runtime cases test function-body permission/GUC fences. No owner SET ROLE is used.

## Closed mutation plans

| Enum | Exact required MODE | Function bytes at base | Required original anchor counts |
| --- | --- | --- | --- |
| `drop-catalog-fence` | `focused:^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$/^authorize$` | 2548 | 2 |
| `drop-bind-catalog-fence` | `focused:^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$/^bind_authority$` | 2985 | 2 |
| `drop-adjustment-orders-fence` | `focused:^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$/^read_adjustments_authority$` | 1665 | 1, 1 |

The legacy drop-catalog-fence constants, exact authorize mode, two anchors and generated SQL change remain the same. The new binder plan changes only its two merchant_access_denied catalog fences. The reader plan changes only its resolve_access orders:read and merchant_access_denied orders:read to store:read. Every selected anchor is validated before constructing a change. Unique start/first exact terminator closes the function body; unchanged prefix/suffix are asserted and hashed in the receipt. The original one-file atomic replacement, clean tracked GitHub workspace, HEAD byte equality, regular/non-symlink path, exact explicit dispatch/CI/BUNDLE=false/no-prebuilt/single-enum EXTRA_ENV and competing-enum checks remain. Unknown enums or mode/combination mismatches fail closed. Receipt retains source SHA, before/after hashes, matches and patch hash; it additionally records exact anchor hashes and prefix/suffix proof. No workflow change is required.

**No mutation script was imported or executed.** Static AST data extraction checked only the closed literal plans and counted original SQL anchors; no replacement was applied, even in a disposable checkout. Source SQL remains byte-identical. Future required raw-log RED witnesses are the binder `missing_catalog_read` direct target SQLSTATE assertion and reader `missing_orders_read` direct target SQLSTATE assertion. Each mutation must run separately, followed by clean source GREEN at the same harness SHA; unrelated failures/top-level PASS alone do not count.

## Preservation and file hashes

The entire base Go file is recovered byte-for-byte by removing only the inserted two-matrix block and appended two helpers. Consequently all original assertions, ordering, authorize ten leaves, twelve prior direct INSERT/bind negatives and twenty-one prior positive leaves remain unchanged. This is source preservation, not a new execution result.

| Path | Base SHA256 | Candidate SHA256 |
| --- | --- | --- |
| `tests/foundation/manual_discount_test.go` | `67cd715a0d0f92c7d87d51d48b785a1c1b36925f014471a2f1a7bc2b429ad241` | `82fc3e623a43f865605c88c09fe1c31dcca5e0de4750993e559e481fe48aa256` |
| `tests/foundation/manual_discount_mutation.py` | `0974d1d6ec9b14995fe52da2be4b063e50a032220ed90db62e57696edcd683fa` | `c26d31514c56b001b0897af57c8f7eb14c25eda2edef1c9f0fe70a5d9449ecce` |
| `migrations/0220_manual_order_discount.sql` | `a0cb0912b9dcf1830e487c66e4060a96d7005b80118adf57ef45ead5d0b6293a` | `a0cb0912b9dcf1830e487c66e4060a96d7005b80118adf57ef45ead5d0b6293a` |

No diff exists under internal/, migrations/, contracts/, .github/ or scripts/. The code graph was consulted read-only; the project map was incomplete for this module, so current source and exact hashes supplied the evidence.

## Actual command receipts

All commands used the assigned worktree except the adjudication read/hash, which used its absolute root path. Tool transcript is the original receipt.

| Actual command | Exit / observed result |
| --- | --- |
| pwd; git status --short; git rev-parse HEAD; git branch --show-current (separate invocations/read groups) | 0; assigned tree, initially clean, base SHA above |
| cat AGENTS.md; sed architecture 14:100, 869:980 and 932:980; cat contracts/tasks.json and invariants.json | 0; initial large response truncated; related tasks/invariants re-extracted with Python JSON |
| sed adjudication ranges 1:120, 120:230 and initial cat; shasum -a 256 absolute adjudication path | 0; full original read across ranges, supplied SHA matches |
| Targeted sed/rg of manual_discount_test.go, mutation script, 0220, tcvEnv member/newBuyer/topUp, moBeginInOtherStore, platform.WithScope and schema grants | successful targeted reads 0; nonzero searches listed below |
| gofmt -w tests/foundation/manual_discount_test.go | 0, after each edit |
| go vet ./tests/foundation | 0 before and after schema/OID clarification; no tests execute |
| go test -c -o /dev/null ./tests/foundation | 0 before and after schema/OID clarification; test binary compilation only, no TestMain/test/PG execution |
| python3 - [AST constant/closed-plan extraction; original SQL anchor counts; byte-for-byte old Go reconstruction; SHA256; forbidden-path diff check] | 0, repeated after final source edit; assertions passed |
| gofmt -l tests/foundation/manual_discount_test.go | 0, empty output |
| git diff --check | 0 |
| Git local commit with core.hooksPath=/dev/null | receipt and final SHA reported in parent handoff after this report is frozen; hooks disabled to obey the no-runtime card |

Failed exploratory commands retained:
- rg referenced absent tests/foundation/main_test.go while finding TestMain in foundation_integration_test.go: group exit 2; correct existing source subsequently read.
- zsh wildcard tests/foundation/transport_convergence* had no match: group exit 1; replaced with rg over tests/foundation and explicit taiwan_cvs_env_test.go.
- shasum used the root-only adjudication relative path from the worktree: group exit 1 after successful static checks; absolute-path shasum subsequently exited 0 with the supplied hash. No acceptance assertion was weakened.

## Remaining obligations / limits

- Both new matrices, actual schema/role catalog, exact HTTP/PG behavior, no-data and unchanged-state runtime results: NOT_RUN.
- Both new target-specific mutation RED, restore/clean GREEN, final normal CI and independent security/source review: NOT_RUN; root retains dispatch/push/merge authority.
- This is a tests-only candidate addressing P1 necessary test gaps, not a product fix or approval. Existing NOTFINDING candidates 1/4/5 were not reimplemented.
- Shared Humaux store/canvas/index acknowledgments are reported in handoff; BUSY must be handed to root for storage.
- No provider/browser/PG service, fixture database, container or port was started. Compile/vet sessions ended. No task temporary file was created. Worktree and evidence remain for root review; root owns/release its author lock.
