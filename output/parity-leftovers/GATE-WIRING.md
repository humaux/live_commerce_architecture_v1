# PR473 runnable gate wiring handoff

- task_id: `53ea9f21-3f3b-4626-87ab-42dc853507b9-sub-leftovers-gate`
- parent_coord: `53ea9f21-3f3b-4626-87ab-42dc853507b9`
- role / dispatch model / reasoning: `test_worker / gpt-6.1-sol / high`; no delegated agents or external model calls.
- base_commit: `33d5749aa597d3ff587225dc580339a80e55f5e5` (original PR473).
- integration: clean merge of `origin/r3/integration` at `c52b65283a5ac738d0c527a464d0cccc9523b1f4`; merge checkpoint `6304373dc842ae0e29401dec5a71d6bf51401ee7`.
- branch / worktree: `codex/pr473-gate` / `.worktrees/codex-pr473-gate`.
- evidence level: **E1, wiring structure only**. Browser/product acceptance, Go compile, real PG runtime, independent review, providers and LIVE are **NOT_RUN** in this subtask.

Changed paths (this wiring commit only):

1. `tests/foundation/browser_parity_leftovers_test.go`
2. `scripts/dev/test-local.sh`
3. `docs/delivery/GATES.md`
4. `output/ci-select-backend-browser/browser-ownership.json`
5. `output/parity-leftovers/GATE-WIRING.md` (existing tracked directory spelling; the assigned uppercase path resolves here on this case-insensitive volume)

The new Go test reuses the signed mock OIDC + production Next + real identity/authorization stack (`brfStartAdmin`) and generated direct `testMatch` (`brfPlaywright`). It registers **the original twelve** desktop/mobile zh-TW checks; `tests/admin/parity-leftovers.spec.ts` is unchanged (SHA256 `c03599eccc37476d114e6afb2a528a9f6f3c92e1bc9ef9861cbba2a5a9b1befe`). There is no root Playwright configuration edit or fabricated UI. The existing live-settings mode retains its tests, env, builds, browsers and 780-second Go timeout.

GET-only MOCK projections cover the exact order/product IDs used by the spec, current valid order/list/actions/history, product+variant, empty valid image list, collections/warehouses and six signed template metadata rows. Each intercepted read uses `platform.WithScope` against the real isolated fixture; other methods/routes fall through to the existing Go handler. The fixture emits no template bodies, provider sends or writes. Before browser execution the new test runs current production DTO parsers with Node and obtains authenticated 200 GET receipts, to identify fixture failures separately from product assertions.

## Actual static commands and exit codes

| Command | Exit | Observed result |
|---|---:|---|
| `git worktree add .worktrees/codex-pr473-gate -b codex/pr473-gate 33d5749aa597d3ff587225dc580339a80e55f5e5` | 0 | isolated worktree created; old parity-leftovers worktree untouched |
| `git merge --no-edit origin/r3/integration` | 0 | clean merge at checkpoint above |
| `gofmt -w tests/foundation/browser_parity_leftovers_test.go` | 0 | Go syntax/format; does not typecheck |
| `node --check --input-type=module` (stdin: extracted generated fixture JS, Go `%q`/`%s` placeholders replaced with syntax-only strings) | 0 | generated fixture JS syntax valid; no fixture logic executed |
| `node scripts/dev/derive-browser-ownership.mjs` | 0 | wrote ownership for 55 modes |
| `node scripts/dev/derive-browser-ownership.mjs --check` | 0 | no ownership drift; existing mode owns original spec |
| `bash -n scripts/dev/test-local.sh` | 0 | shell syntax valid |
| `git diff --check` | 0 | no whitespace errors |
| `python3 scripts/check_packet.py` | 0 | `PASS_PACKET_STRUCTURE_ONLY`, `delivery_baseline_checked=false`; generated `experiments/results/packet-check.json` restored to checkpoint bytes afterward |
| `shasum -a 256 tests/admin/parity-leftovers.spec.ts tests/foundation/browser_parity_leftovers_test.go` | 0 | original spec hash above; new Go source `f9b477fc6c447a665e2712fddd3434a1602e1d160134048453bbd94d4301b0c9` |

No local tests, browsers, servers or containers were started. No production implementation, assertions, thresholds, dependency locks, contracts or CI definitions changed. No dependency-map regeneration is needed: the new file is a browser-tagged test only and the current depmap reads non-test package imports.

## GitHub gate to run after parent review/push to existing PR473

`bash scripts/dev/test-local.sh --browser-live-settings`

Its native runner is:

`LC_BROWSER_LIVE_SETTINGS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.3 go test -race -tags browser -count=1 -timeout=780s -run '^(TestBrowserLiveSettingsUIRealChain|TestBrowserParityLeftovers)$' -v ./tests/foundation`

The shell mode provides the existing PG/build/browser preparation. Do not claim a product RED from gate registration, Go compile, login, fixture parser, API fixture, browser launch or timeout failures. Parent must read actual GitHub Playwright enumeration of all twelve cases, successful signed login and fixture logs before classifying assertion failures. No new/duplicate PR or remote push was made by this worker.

Expected run-owned artifacts: `${LC_BROWSER_EVIDENCE_ROOT:-output/playwright}/parity-leftovers/<UTC>/fixture-contract.log`, `mock-projections.json`, `mock-get-receipts.json`, `next.log`, `playwright.log`, generated Playwright config, failure screenshots/traces and original `parity/clicks-*.json`. These paths are **NOT_RUN**, not existing runtime evidence.

## Scenario classification and unresolved limits

| Original scenario | Fixture readiness / authority boundary |
|---|---|
| 訂單缺少收件資料 (2 cases) | **BLOCKED data/route contract alignment**: current frozen order DTO requires a complete destination and represents an existing order. The neutral valid DRAFT fixture does not model an uncreated pre-order with missing phone/CVS information; failures here are not sufficient product RED. Root authority must define that data/route contract before implementation. |
| 訂單匯出說明 (2) | Current order GET/actions ready as MOCK; missing export help route/copy can be classified only after actual CI login/fixture receipts. |
| 媒体上传 (2) | **BLOCKED media candidate contract alignment**: current image list is valid empty and has no signed candidate name/label field or shared candidate library route. Do not call missing named candidates product RED, invent fields, or add fixture HTML. Product media controls/copy still execute unchanged; classify actual failures from CI. |
| 批量填充 (2) | Valid current product+variant MOCK GET; actual controls/copy assertions remain original and await CI. |
| 消息模板 (2) | Six valid metadata records carry the exact signed names/versions/kinds/dates/public flags; routes/copy await CI. Existing repeated exact-text selectors remain unchanged; classify ambiguity as an original-test issue if observed. |
| 新增自訂訊息範本 (2) | GET metadata available; route/form is real product code. No publish/send fixture action is implemented; original visible-copy assertions await CI. |

The parser preflight and authenticated raw GET preflight are newly authored and await their first GitHub execution. The original 780-second combined budget is unchanged; budget sufficiency is UNKNOWN until observed. Parent owns independent review, push to existing PR473, CI monitoring and any narrowly authorized fixture repair (maximum two directed attempts). No product implementation is authorized by this handoff.
