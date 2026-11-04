# PR394 direct authorize database matrix — source handoff

Status: **E1 / SOURCE, runtime NOT_RUN**. This is new coverage predicted GREEN, not a reproduced current product vulnerability and not acceptance completion.

- task_id: `ae69dcbd-5538-447a-a876-56db3b293047`; root-owned coord and author lock; child `/root/authorize_matrix_1048`, own canvas `ae69dcbd-sub-1`. No separate claim or delegation.
- base_commit: `81044a92a7ba1823ec40f75e067e593103e764bc`.
- branch/worktree: `codex/pr394-offline-diagnostic`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr394-offline-diagnostic`.
- requested model/reasoning: Sol/high; actual model/provider ID: **UNKNOWN**.
- write_paths: `tests/foundation/manual_discount_test.go`; this report only. No production SQL/Go, gate, grants, fixture helper or migration changed.
- test source SHA256: `67cd715a0d0f92c7d87d51d48b785a1c1b36925f014471a2f1a7bc2b429ad241`. Commit and report digest are supplied by the local git/hash receipt after this report is written.

Authority read: repository AGENTS; architecture sections 0/1/2/26/27; T03/T04/T06/T11 task DAG; invariants I01/I02/I08/I18/I21; `contracts/cart-quote-v1.md` amendment §G3; `migrations/0220_manual_order_discount.sql:64-97`; actual platform scope and buyer/checkout fixture helpers. Review locator read in full: root `output/integrator/tools/logs/ci-evidence-1238/review394-db-810-1035.md`. Humaux `4dd122d2-dd4d-4df7-9a5a-ce0125e6738c` and current development rules `7bdb220b-0798-4b1d-aff5-16ee94cae75f` read in full; rejected enumeration 11/11, coverage 1.0. Initially missing Humaux tools became available; no local/platform memory read or write occurred.

## Source coverage manifest

New root (Go normalizes spaces to underscores):
`TestMerchantToolsManualOrder/FEAT-MANUAL-DISCOUNT/runtime_database_boundary/authorize/`

| Suffix before Go space normalization | Target/source boundary | Required result |
|---|---|---|
| neither permission | resolve_access + dual merchant fence | PT403; zero owner authorization rows |
| missing inventory reserve | resolve_access + dual merchant fence | PT403; zero rows |
| missing catalog read | dual merchant fence | PT403; zero rows |
| both permissions persist unbound authorization | direct successful SQL insert | success; exactly one row; every field below matches |
| tenant GUC mismatch | app.tenant_id scope fence | PT403; zero rows |
| store GUC mismatch | app.store_id scope fence | PT403; zero rows |
| principal GUC mismatch | app.principal_id scope fence | PT403; zero rows |
| mixed buyer GUC | app.buyer_id exclusion | PT403; zero rows |
| commerce_buyer_runtime EXECUTE denied | actual buyer runtime pool function ACL | 42501; zero rows |
| commerce_checkout_runtime EXECUTE denied | actual checkout runtime pool function ACL | 42501; zero rows |

**10 new leaves: 9 negative + 1 positive.** All existing 12 direct-INSERT/binder negatives and existing 21 positive assertions are preserved: removing only the new block reproduces the entire base test file byte for byte. Existing in-body controls also remain unchanged. No deleted/relaxed assertion or fixture exists.

The seven merchant denials and success control enter the existing genuine restricted `f.runtime` through valid `store:read` `platform.WithScope`; the existing callback witness checks derived tenant/store/principal, and each leaf separately witnesses the direct function call being reached. Actual stored grants are independently counted with the owner readback and compared with the runtime permission checker before the target; these checks fail the leaf instead of serving as its expected denial. GUC attacks use real existing other tenant/store/principal or the actual synthetic buyer owner, after both permissions are witnessed. `set_config` failure is fatal. SQLSTATE must be exactly PT403, so setup/FK/Go precheck errors cannot pass.

Success commits and reads back `tenant_id`, `store_id`, `owner_id`, `principal_id`, exact request hash bytes, UTF-8 name `直播優惠`, amount `100`, and `quote_id IS NULL`; expected scope comes from the real fixture and server-resolved callback. Authorization counts are owner-wide (including other scopes), preventing a misplaced inserted row from escaping readback.

ACL leaves borrow `b.h.a.runtime` and `e.p.pool` already opened by the real fixture. They start native-role transactions; no SET ROLE, forged role or GUC is used. In the same transaction they require current_user=session_user, intended role membership, no merchant/auth/superuser/BYPASSRLS authority, schema USAGE, and actual `has_function_privilege(...,'EXECUTE') = false`, then directly call the function with valid typed arguments and real owner. They require SQLSTATE 42501 and rolled-back zero rows. An unexpected grant fails independently before the call. Schema USAGE is explicitly required so a missing schema privilege cannot masquerade as the function ACL result. No new role/pool/grant helper was introduced.

## Actual commands and results

Worktree initial `git status --short` was empty. All runtime/build commands remain NOT_RUN.

| Actual command / inspection | Exit / evidence |
|---|---|
| `pwd`; `git rev-parse HEAD`; targeted `cat`, `sed`, `rg` source/authority reads | 0 for completed read commands; exact base above |
| Initial `rg ... db/migrations/0220*` | 1: zsh no matches; corrected to existing `migrations/0220_manual_order_discount.sql` |
| Later grouped read inspections | Overall 0; reported missing guessed `buyer_test.go` and `payment_sandbox_test.go` were corrected with discovered `buyer_capability_test.go` and `payment_start_test.go`; not runtime failures |
| Inline Python additive source edit | 0; assigned test path only |
| `gofmt -w tests/foundation/manual_discount_test.go` | 0; parses/formats test source only, no compilation |
| `git diff --check` | 0 |
| Inline Python source-preservation assertion | 0; complete base file byte preserved after subtracting the new block |
| `command -v gofmt` | 0; `/opt/homebrew/bin/gofmt` |

No Go test/build, PG, HTTP/browser runtime, CI/provider/network request, push or mutation was started. No active process, fixture, container or port was created by this task. Existing fixture cleanup remains owned by its existing t.Cleanup callbacks when CI eventually runs.

## Remaining gate and controlled mutation proposal — NOT_RUN

Root must independently execute existing CI g05 at the new exact commit and inspect actual RUN/PASS for these 10 leaves plus unchanged 12 old negatives and 21 existing positives. Existing dispatch uses `LC_FOUNDATION_PKGS=./tests/foundation` with `shard-plan.mjs run g05` / `skip g05` through `scripts/dev/test-local.sh`; the new leaves remain under the existing selected test root. Current-head runtime is UNKNOWN until those receipts exist; prior 810 evidence cannot certify this new file hash.

Minimal permission-fence witness for a separately frozen controlled CI mutation: **only within authorize**, remove `catalog:read` from both calls to `merchant_access_denied`, retaining the existing inventory resolve, table ACL, RLS, Go permission guards and all tests. The new `missing_catalog_read` leaf should enter SQL, insert one row and fail its exact PT403/zero-row oracle; old direct INSERT/HTTP negatives alone would not detect this. Restore exact production source and obtain GREEN at a restored exact head. This mutation was neither applied nor run here. Failure to turn this target RED is a gate failure, never grounds to weaken assertions.

Independent review, runtime compilation/type checking, the controlled RED→restore→GREEN witness and #505 lock-wait revocation remain **NOT_RUN**. #504/#505 are not closed by this source-only handoff. No ordinary HTTP exploit, production grant/state, global PR approval or product completion is asserted. No authority/gate was relaxed to claim completion.


## Frozen CI-only mutation harness source handoff (2026-11-02)

- task_id: `db8ec69c-cde6-4dbd-bc56-fd926ca739fb`; root claimed; child `/root/authorize_matrix_1048`, own canvas `db8ec69c-sub-1`; no separate claim/delegation.
- base_commit: `0b6de820d17971b2eb0e3d4237e4e72275ea14df`; same worktree/branch and root-owned author lock.
- write_paths: `tests/foundation/manual_discount_mutation.py`, additive five-line stanza in `.github/workflows/gates.yml`, this report. Original Go test and every production SQL/Go file remain byte-identical to base.
- evidence level: **E1 / SOURCE only**; injector, Go/PG/browser/CI/provider/network/push/dispatch are **NOT_RUN**. Requested Sol/high; actual model/provider ID **UNKNOWN**.
- authority: root task card plus Humaux human-source freeze `74e9c85e-04de-4600-9c12-a9605277707f` read in full; existing OA injector, gates shell, ci-plan and test-focused source read. Existing 475 import-field injector read as a pattern only; no recipe or closed list was expanded or moved. Ponytail skill applied: stdlib, one source path, one recipe, no generic patch API.

The only accepted fault is `LC_MANUAL_DISCOUNT_MUTATION=drop-catalog-fence`. The source recipe requires exactly two whole `merchant_access_denied` call anchors **inside authorize's single CREATE FUNCTION body** and removes only `catalog:read` from their arrays. It preserves the initial inventory resolve, binder, ACL/RLS, Go guards, immutable tests and every surrounding source byte. It does not modify a live database; the changed SQL will be applied only by the existing disposable PG fixture in CI.

Before any source/evidence write, the injector requires `GITHUB_ACTIONS=true`, `CI=true`, `GITHUB_EVENT_NAME=workflow_dispatch`, exact MODE `focused:^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$/^authorize$`, `BUNDLE=false`, absent prebuilt directory, EXTRA_ENV exactly the single accepted key/value, no simultaneous OA/import-field mutation, GitHub workspace matching git root, all tracked files clean, a regular nonsymlink fixed allowlisted path equal to HEAD, one function anchor and two matching fences in that body, and safe unused evidence destinations. Unknown input/mode, dirty source, path escape/symlink, missing anchor or existing receipt fails before writing. The fixed target is written once with a same-directory temporary file and `os.replace`; no production/source write was executed locally.

Receipt paths follow the existing gate output/upload tree:

- `output/ci-gates/manual-discount-mutation.json`: source HEAD, exact mode/fault/path, expected match count 2, source before/after SHA256 and SQL patch SHA256.
- `output/ci-gates/manual-discount-mutation.patch`: actual `git diff --no-ext-diff --no-textconv -- migrations/0220_manual_order_discount.sql`, also printed in raw CI log.
- existing gate log for this exact mode: `output/ci-gates/focused__TestMerchantToolsManualOrder___FEAT-MANUAL-DISCOUNT___runtime_database_boundary___authorize_.log`. Existing workflow uploads `output/ci-gates/` with its matrix artifact; no artifact name is invented here.

The workflow invokes the script only when the new variable is nonempty and explicitly checks dispatch. Subtracting those five added lines reproduces the complete base workflow byte for byte. Its existing `set -o pipefail`, ordinary commands, verdict propagation, per-mode logging, receipts, calibration prebuilt skip and artifact upload remain unchanged. The script does not catch a test exit or turn expected RED into a successful job.

Actual additional static commands: inline Python source creation and `ast.parse`/literal anchor checks/test byte comparison/workflow subtraction checks all **exit 0**; extracted Gate shell `bash -n` **exit 0**; `git diff --check` **exit 0**. Initial read command ended **1** on the intentionally absent new injector and a nonexistent nested `.worktrees` search; corrected root absolute 475 path read succeeded **0**. No injector import/main or runtime self-test was executed; runtime rejection/atomic-write behavior is source-reviewed only.

### Reviewed dispatch text — NOT EXECUTED

`REVIEWED_REF` below must be set by root to the reviewed branch/tag whose head is the final harness commit, then source SHA in the actual receipt must equal that reviewed head. Both runs use the same ref/head and exact mode; root registers run IDs, deadlines and logs.

```sh
gh workflow run gates.yml --ref "$REVIEWED_REF" \
  -f modes='["focused:^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$/^authorize$"]' \
  -f full_browser_universe=false \
  -f extra_env='LC_MANUAL_DISCOUNT_MUTATION=drop-catalog-fence'
```

Expected mutant oracle: `missing_catalog_read` callback and direct SQL execute are reached, the function actually inserts the authorization, then the unchanged assertion observes empty SQLSTATE plus one row and fails the native PG gate. Root must inspect that exact failure and receipt patch; generic CI/setup failure is insufficient.

After that RED is terminal, run the identical harness source head without injection:

```sh
gh workflow run gates.yml --ref "$REVIEWED_REF" \
  -f modes='["focused:^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$/^authorize$"]' \
  -f full_browser_universe=false \
  -f extra_env=''
```

Require actual PASS for all 10 authorize leaves, no skips, the same original source/test hashes and successful ordinary gate exit. Full existing CI and independent review still apply to the final head. Neither source handoff nor expected fault behavior constitutes a measured RED→GREEN witness; both runs and review remain **NOT_RUN**. No active process/container/port/fixture or temporary source file was created by this authoring task.
