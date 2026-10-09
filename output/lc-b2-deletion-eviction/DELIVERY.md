# LC-B2-DEL PR24 round1 — actual request clock

- Branch/worktree: `unit/lc-b2-deletion-eviction`, `.worktrees/lc-b2-deletion-eviction`; owner packet `output/integrator/triage/pr24-0c8351ba.md`, thread4228903585.
- Fetched/merged own origin: no-op at0c8351ba. Requested trunk/PR22 merge: **bd34d6d114bdabfc2e4a334261d6a8199eda9b5c**, conflict-free; new mode registry retained, `--list` and check-gates exit0. No runner edits.
- Tested source: **7872cabfad6863697a75215006447f48dbb12fe4**; current source/log SHA256 manifests in `source-hashes.json`, `results.json` and `pr24-r1/`. Final commit adds evidence only.
- Author: Codex-4 / GPT-6 family; exact runtime model/effort not exposed. Single writer, bounded read-only clock review, no recursive delegation. Taskf09fa1c9-9e3e-4dce-995e-560e3ad50728.

## Fix

The shared rate gate no longer receives a sweep/caller time argument. It samples one injectable current clock at admission, actual GET start and Graph completion; valid-JSON parse failures use that same clock at parse completion. A goroutine delayed after admission preserves full reserved spacing from its actual GET start. Only the bridge's existing3s elapsed-wait cap uses real monotonic elapsed time/timers.

`SweepOnce` uses the injected clock for scheduling, and each `pollOne` reads it freshly after token loading. Forward, deletion, older and facts reads all use the same gate. `ConsoleConfig.Now` is one optional concurrent-safe Go callback, default `time.Now`, so the real external REAL_PG fixture can inject logical time; no new Clock/timer interface, HTTP route, SQL/migration, protocol or environment flag.

Deletion50-id/60s/30min eligibility, seq/epoch, fail-safe parser/custody, data clearing and the prior backoff/usage ladder remain. Existing scheduling tests now advance the injected clock instead of merely passing a timestamp; their call-count/no-eviction assertions remain. The previous20ms slow-response test keeps its exact oracle with a deterministic20ms logical advance; the facts404 bridge-wait test uses an advancing clock, preserving its real bounded waiting behavior.

## RED → GREEN / gates

Exact commands and exit codes: `results.json`. All RED logs are uncompressed, exact plaintext.

| Gate | Exit | Evidence |
| --- | ---: | --- |
| Unit sequential/admission counterexamples before clock wiring | 1 | Both top-level tests fail; later B makes early GETs for429,50/80%usage, forward/deletion parsing; `pr24-r1/unit-red.log` |
| Actual REAL_PG SweepOnce before clock wiring | 1 | Both429/80%cases fail to reopen at the injected actual completion deadline; `pr24-r1/pg-red.log` |
| All focused `^TestComment` cases after fix | 0 | `pr24-r1/unit-green.log` |
| Actual REAL_PG multi-asset SweepOnce after fix | 0 |1 top-level PASS, both429/80%cases; `pr24-r1/pg-green.log` |
| `GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=120s -v ./internal/integrations/metareply ./internal/integrations/meta/oauth` | 0 |**61 PASS /0 FAIL**, package.log |
| `bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN(01\|02\|04\|05)' ./tests/foundation` | 0 |**10 PASS /0 FAIL /0 SKIP**,44.129s, pg-regression.log |
| `bash scripts/dev/test-node.sh` | 0 |**1181 PASS /0 FAIL**, node.log |
| `GOTOOLCHAIN=go1.27.2 go vet ./internal/integrations/metareply ./internal/integrations/meta/oauth ./cmd/claims-worker` | 0 |vet.log |
| `pnpm --filter @live-commerce/admin typecheck` | 0 |typecheck.log |
| `bash scripts/dev/test-local.sh --list` | 0 |registry-list.log; registry layout kept |
| `bash scripts/dev/check-gates.sh` | 0 |82 documented modes; headers/tracked-test coverage green |

**E3: MOCK + actual SweepOnce REAL_PG**, bound to source7872cabf. Two distinct assets run sequentially: the first response advances10s, the second3s. Real Page demand, registered/decrypted tokens and lease rows are exercised. B's actual GET count remains unchanged through completion+delay−1ns and increases exactly at the deadline (2s429 /15s80%usage). Unit tests additionally cover5s50%usage, both parse-failure paths and slot admission. Logical delays require no wall-clock sleeps; no deletion semantics are mocked away.

Diagnostic history is retained: first PG fixture omitted page demand and hit intentional idle-drop; real demand was added. The first malformed-forward fixture was a missing-data envelope which the existing normalizer accepts as empty; it was changed to a data object to exercise the intended parse-error branch. Neither adjustment removes or weakens a business assertion. The injection seam existed but was unused before the behavioral RED runs.

Bounded independent source/log review at7872cabf: PASS, no confirmedP0/P1/P2, runtime independently NOT_RUN; review memory07df7ac6-301e-4b00-b3a7-2aa22746ca4c. All owned race/PG/Node runners and fixtures completed cleanup; no push or production action. Earlier initial/K3-P2 manifests remain explicitly historical in `initial-*` and `k3-p2-*` records.

## CI / limits / next

Needed CI: foundation-shards including the new actual LCN01 sequential-clock case and metareply/oauth race tests, plus the required mode-planner set. Full foundation/browser/provider SANDBOX/LIVE/production and new-head K3/CI are NOT_RUN locally under owner RAM rules. No real credentials, buyer PII, messages or money used.

Original deletion capacity bound remains: stable N takes ceil(N/50) admitted cycles plus budget/provider delays;2000 entries need40 rounds (~40min), exceeding the30min eligibility window. Optional actual Graph deletion browser fixture remains NOT_RUN because its business-MOCK harness does not wire the actual poller.

After this priority handoff, return to cart-refresh-generation (84465513), merge PR22 trunk there and verify the registry before any next commit. Integrator reviews/pushes; author commits and stops. Only typecheck's final blank EOF line is trimmed in the committed review log; canonical raw stdout and both hashes are recorded.
