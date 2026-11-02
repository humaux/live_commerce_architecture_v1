# PR475 F03 tests-first counterexamples — E1 / runtime NOT_RUN

- task_id: `efe7f48f-b259-4f84-874e-ccaa46e69a48`; writer/canvas `efe7f48f-sub-1`; parent owns claim and author lease. No separate claim/delegation.
- base_commit: `8d655d16d6c929f3eceb70dfa09ac3dccbcc7f31`; worktree `.worktrees/codex-pr475-inventory-repair`; branch `codex/pr475-inventory-repair`.
- Role: tests-only author; assigned Sol/high; actual model/reasoning telemetry **UNKNOWN**. Independent acceptance is parent/CI work, not this author's static result.
- Allowed/changed paths: `tests/admin/parity-customer-import.spec.ts`, `output/parity-data-ops/F03-RED.md` only.
- Evidence class: **E1 syntax/source/byte preservation**. Browser, PG, provider, CI, typecheck and causal RED/GREEN **NOT_RUN**. No product repair or visual PASS claimed.

## Authority and input derivation

Read human freeze `543372fe-e07c-4e9a-9b4b-1e538e98c166` in full; fixed-base `AGENTS.md`, architecture §§0/1/2/26/27, tasks T14/T18/T21, invariants I01/I02/I05/I06/I09/I11/I18 and `contracts/migration-import-v1.md` (SHA256 `569a88fba4666550499d746af2ff567338cafb11170fe6d6708b1b7c3cad072f`). Rejected enumeration:11/11, coverage1.0. Existing Humaux code map/search located `ImportVerdictTable`; local source is execution authority.

Read parent-reviewed `output/integrator/tools/logs/ci-evidence-1238/475-f03-source-1555.md` (SHA256 `f16cc591f888b26188ecdc665e0646c9ba02e5aae5168304d44d12c6a3cb54b3`) as a navigation aid, then inspected actual leaf code/model, original tests, FRAME-COPY preview660–715/stale888–950, and readonly fixture snapshot shape. Official desktop PNG hash/provenance is in that SOURCE card; this author did not re-render/view new pixels. Saved official desktop1028:6685 hash `295b87cccce672ebb9f9d50adb87ae44692250ce3c8a32b17c072190d5dff0d9`,1198:10518 hash `78664bac3c3fd182bb3efac89cb7494ad794f1c19db68cb503ac5e8c2925ff7a`, exported2026-11-02 11:50/11:52UTC. New runtime visual state remains UNKNOWN.

The unseeded input keeps the old synthetic three-row CSV shape: distinct valid prefixed source IDs A/B, valid names, blank optional phones; third nonblank row lacks source ID. Contract§3 and `customers_csv.go:225–244` reject row3 as `required`; `migrations/0152_customer_import.sql:201–224` creates an owner for each new ID, whereas an existing ID updates. Before uploading, each new test checks both IDs absent in fixture owners. Thus independent expected counts are **new2/update0/apply2/failed1**, with source response order1/2/3 and rendered failure-first order3/1/2. This is source-derived expectation, not a reported runtime result. Every test also asserts the actual preview DTO/counts and visible totals before layout checks. The old seed and1/1/1 expectation remain byte-identical.

## Implementation and runtime click ledger contract

- Six independent tests: en/zh-TW/zh-CN ×1440/390. Each uses existing MOCK owner/OIDC sign-in, native filechooser, three native selectOption controls and actual Preview button. No direct import seed/commit or handler change.
- A local helper checks three rows, exact safe localized composite summaries plus independent nonempty text, stable row IDs/order, original html lang and horizontal-overflow<=1. It reads each composite cell/row edge and requires both absolute deltas<=2CSSpx. A separate step soft-asserts each pager's actual visibility equals false. Immediate boolean visibility checks ensure a geometry RED cannot consume retry time and prevent observing the pager RED.
- Native wheel movement uses read-only table coordinates to reach the row card. DOM evaluation only reads geometry/attributes. No force click, dispatchEvent, DOM write, swallowed failure, skip, expected-failure, timeout change or shared-helper change.
- Existing zh-TW preview and stale cases at both widths call the helper only **after their original captures**. Seed count, stale MOCK409 body/single attempt, request hash/UNKNOWN/privacy/amount/capture/wait assertions remain intact. Other existing outcome cases receive no new helper call.
- Each new test records zero commit requests across the flow and compares existing readonly state before/after: owners/profiles/history/batches/consents/receipts/audits counts, complete owner map and batch array. It checks all synthetic private markers absent from main. Existing `action`/`click` support writes the actual click ledger at runtime; new attachment contains only safe row IDs, edge deltas and pager booleans.
- Exact en/zh-CN signed pixel/copy approval remains UNKNOWN; existing copy only supplies functional control/summary labels. Mobile below-fold visual approval and full44-frame/tokenDiff gate remain unresolved.

## Static verification, commands and exits

All commands ran in the assigned tree unless the SOURCE card path is named. No server/process/container/fixture/browser was started, no dependency installed, no provider call, push or dispatch.

| Actual command/action | Exit/result |
|---|---|
|`git rev-parse HEAD`; `git status --short`; `git ls-files --others --exclude-standard`; `git rev-parse --abbrev-ref HEAD`|0; fixed8d655, trackedclean initially, exactly five prior untracked files|
|Bounded `sed`/`rg`/`cat` and inline Python reads of AGENTS/architecture/tasks/invariants/contracts/components/model/copy/tests/fixture|0 for corrected source reads; two exploratory wrong file paths/unmatched zsh globs returned1; no mutation|
|`test -x node_modules/.bin/tsc`|1; no local compiler/dependencies|
|`command -v tsc`|1; no available compiler; typecheck NOT_RUN, no installation|
|`node --experimental-strip-types --check tests/admin/parity-customer-import.spec.ts`|0, syntax only, no test execution/import evaluation|
|`git diff --check`|0|
|Inline Python exact inverse-removal byte gate below, allowed-path/hash checks|0; original15,940bytes restored exactly; five KEEP unchanged; old import-wizard unchanged|
|Humaux own-element canvas update|ok; dispatch lock acquired=true, finally released; parent element/author lease untouched|
|Browser/CI/Go/PG/platform/provider/push/dispatch|NOT_RUN; explicitly outside this card|

Spec SHA256: `359e317b7a8b82769c551d5d0548c70380ad69179d266a35204b4ec1bb7172ae`.
Exact spec diff SHA256 (`git diff <base> -- tests/admin/parity-customer-import.spec.ts` before commit): `118c5343596f4411021bd3666d2c54ec55816d9ee5ce755402dd45e25554fd53`.
Original spec SHA256: `b68817c2d7e419804a622fe0645728624fcd77dec21c71b77e6e978d07d19b22`.
Readonly `tests/admin/import-wizard.spec.ts` SHA256 unchanged: `393c865133fd11672c478d94709cd8c163e1fc9d4463e06fedca960645387bf7`; original5000-row Next/Previous50→51→1 and1280/PII oracles unchanged.

Reproduce byte gate (reads only; no AST/count substitute):

```sh
python3 - <<'PY'
from pathlib import Path
import re, subprocess
p = 'tests/admin/parity-customer-import.spec.ts'
old = subprocess.check_output(['git', 'show', '8d655d16d6c929f3eceb70dfa09ac3dccbcc7f31:' + p])
restored = Path(p).read_bytes()
for name in ['imports', 'preview-check', 'stale-check']:
    restored, count = re.subn(rb'^[ \t]*// F03-BEGIN ' + name.encode() + rb'[^\n]*\n.*?^[ \t]*// F03-END ' + name.encode() + rb'\n', b'', restored, flags=re.M | re.S)
    assert count == 1, (name, count)
restored, count = re.subn(rb'\n// F03-BEGIN tests\n.*?// F03-END tests\n', b'', restored, flags=re.S)
assert count == 1
assert restored == old
print('BYTE_GATE PASS', len(old))
PY
```

Original five untracked KEEP SHA256, independently checked before/after:

| File under `output/parity-data-ops/inventory-diagnosis-4556/` | SHA256 |
|---|---|
|13081478979-error-context.md|5a5e42926305ea21b218660b7c3aeee727d6780999ee5c0c51c957671b325d7f|
|13081478979-test-failed-1.png|89f9c4d866a39e7bc23daea35126b937f76e4c5c0e084e9c84433becf62b311a|
|13081974565-error-context.md|075ff7e721d9a30d00aefc226d25a50d10e563b0d46a5912a1a3d0e327eda8ac|
|13081974565-test-failed-1.png|99abbbe83f2acba27798441a58ae136db54fece4d29b2cb5ae40e08cbea2085a|
|trace-summary.json|8169a244667615572954faf70421f3b5c8f0043348c019a667d8478ba606f25e|

## Required next evidence / unresolved

1. Parent must independently audit diff/byte gate, then CI must reach both precise geometry/pager oracles in all six new previews and original preview/stale width pairs. Predicted current8d RED: composite summary right edge exceeds2CSSpx, and both three-row pager buttons are visible. **Actual RED NOT_RUN**; setup/count/privacy failure alone does not establish F03 RED.
2. No product candidate is authorized by this author's E1 result. Preserve real causal RED before the separately frozen two-leaf product repair, then run exact same checks GREEN plus original high-row/amount/privacy/UNKNOWN suites.
3. Full44-frame visual/tokenDiff acceptance, en/zh-CN signed-copy authority, mobile pixels, LIVE and production approval remain UNKNOWN/NOT_RUN. No standards weakened and no failed evidence removed.
