# cart-refresh-generation delivery

PR22 follow-up: merged current trunk conflict-free as **ae17bc99fcb78ef573b4eaa48f475fc2d8fae562**. The native mode registry is retained; `bash scripts/dev/test-local.sh --list`, check-gates, Node and storefront typecheck all exit0 after merge. Current Node total **1189 PASS /0 FAIL**. Exact post-merge evidence: `pr22-merge/results.json`. No diff in apps/internal/migrations/tests/foundation; previous browser32+13 evidence remains bound to identical runtime/source bytes. No browser rerun claimed. Previous source/gate rows below describe the original84465513 handoff.


- Branch/worktree: `unit/cart-refresh-generation`, `.worktrees/cart-refresh-generation`; base **ddba31c9** (trunk after #16). `git fetch origin && git merge origin/r3/integration` → exit0, no-op before RED.
- Tested source: **3b3350f8aaf7a4d586229cf492558bc59c6432a3**, bound by `source-hashes.json`. Final commit adds evidence only.
- Author: Codex-4 / GPT-6 family; exact runtime model/effort not exposed. Single writer of the two source/test files; read-only state reviewer and browser-prep collaborator, no recursive delegation. Taskee53009c-a14b-4d47-a611-1d43915c3f4a.
- Brief: `output/integrator/triage/brief-cart-refresh-generation.md`. No API/Go/SQL/contract/copy/dependency/lockfile change.

## Result

`CartProvider.tsx:65` now separates issued tickets from the latest **successful session observation**. A newer failed session read cannot invalidate an earlier observed cart. Only a newer successful session—including absent—or an explicit command/unmount fence supersedes it. Cart completion still checks its observation ticket, current context and working flag; all old command checks and same-receipt Retry behavior remain.

The actual component + real session/purchase/journal clients run in the existing Node VM harness with only network/browser primitives mocked. Two transport/503 counterexamples went RED→GREEN. Controls cover a newer B session with a failed cart read, valid delayed A context validation, successful absent session, completed same-context command versus old cart, and actual effect cleanup versus late session/cart state writes. New cases await exact refresh promises or network events; no fixed tick count. Six existing test bodies are unchanged. No new visible control was introduced.

## Commands / evidence

Exact commands, exit codes and log hashes are in `results.json`; all logs are uncompressed plaintext; exact canonical RED stdout and its normalized review-copy hash are both recorded.

| Gate | Exit | Result |
| --- | ---: | --- |
| `pnpm install --offline --frozen-lockfile --ignore-scripts` | 0 | Existing frozen packages; `install.log`, lockfile unchanged |
| `node --test --experimental-strip-types apps/storefront/tests/cart-provider-retry.test.mjs` before fix | 1 | **2 FAIL /8 PASS**; both valid carts discarded after newer session failure, `red.log` |
| Same focused Node command after fix and added safety controls | 0 | **14 PASS /0 FAIL**, `green.log` |
| `bash scripts/dev/test-node.sh` | 0 | **1165 PASS /0 FAIL**, `node.log` |
| `pnpm --filter @live-commerce/storefront typecheck` | 0 | `typecheck.log` |
| `bash scripts/dev/check-gates.sh` | 0 |82 modes documented, tracked suites assigned, headers green |
| `LC_BROWSER_ENGINE=chromium LC_TEST_LOCK_WAIT=300 bash scripts/dev/test-local.sh --browser-order` | 0 | **32 cases**, real UI; PG exact facts:6 test buyers/7 orders, `browser-order.log` |
| `LC_BROWSER_ENGINE=chromium LC_TEST_LOCK_WAIT=300 bash scripts/dev/test-local.sh --browser-buyer` | 0 | **13 cases**, real browser/Next/Go/PG,1 order/hold, `browser-buyer.log` |
| `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` | 0 |52 conservative CI modes; deploy=false, `pr-modes.json` |

**E3: tested component/client MOCK + Chromium/Next/Go/REAL_PG environment**, current source3b3350f8. These browser modes are the focused tagged Go harnesses `TestBrowserBuyerOrderUI` and `TestBrowserBuyerRealChain`; no production Go was edited. Browser runs were strictly serial on fixed source. Build lock released; owned browser/HTTP/PG fixtures completed their normal cleanup. Tracked `next-env.d.ts` and all runtime source bytes stayed unchanged; no unrelated generated files staged.

Click evidence: **21** real cart UI ledger rows in `cart-click-ledger.json`, plus Go-verified `cart-final-pg-facts.json` / `cart-retry-facts.json`. `browser-order-evidence.json` and `browser-buyer-evidence.json` list screenshot/log/result hashes and canonical primary paths. Full artifacts are copied uncompressed to `output/cart-refresh-generation/browser/order/` and `/buyer/` in the primary checkout, so worktree cleanup does not discard them. The precise failed-refresh race is proven by Node; browser modes prove ordinary real UI/cart/write/Retry regressions.

Independent bounded source/log review at3b3350f8: PASS, no confirmedP0/P1/P2; original six test bodies checked byte-identical. E1 review only, not an independent runtime rerun; memoryd066606a-019a-48d5-85ae-95b32f844670.

## CI / NOT_RUN / risks

The shared storefront component conservatively selects **52 modes**. Both brief-required browser modes ran locally; the remaining50 (including full foundation and other browser modes) are **NOT_RUN locally**, retained for integrator CI under the owner RAM rule. Full exact list is `pr-modes.json` / `results.json`; no planner mode was removed to reduce coverage. This is a local author handoff, not whole-PR/production acceptance.

PR CI/K3 review on the new head, independent runtime repetition, other browsers and live provider/production acceptance are NOT_RUN. No real buyer PII, messages, money or live credentials used. No runtime blocker found. Commit and stop; integrator pushes.

Only typecheck stdout's final blank EOF line is trimmed in the committed review copy; raw canonical stdout and both hashes remain recorded. Canonical RED stdout is exact and uncompressed; only whitespace-only Node error-diff padding lines are stripped in the committed review copy, with both hashes recorded. Staged-evidence check-gates exits0.
