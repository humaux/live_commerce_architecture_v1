<!-- Purpose: Hand off the LC-U1 UI candidate and exact GitHub-only acceptance gates without claiming unrun browser results.
Depends on: contracts/live-console-v1.md LC-U1/Amendment 1, source 975ed53f, trunk 0617bc40 and CI 37567262145.
Used by: The integrator pushing unit/lc-u1-shell and running .github/workflows/gates.yml; independent review. -->
# LC-U1 — CI5 sweep lease/status-heading repair; GitHub rerun pending

Latest source **`975ed53f28b15b2bd78ba767c56092236004fc40`**, trunk `0617bc40` merged. [CI5-DELIVERY.md](CI5-DELIVERY.md) records the 96-failure diagnosis and repair: one-hour signed test session/synthetic domain versus an 85-minute gate that ran 64 minutes; safe status headings; actual shared-component SSR title/singleton-nav/privacy guards. Node **527/527**, SSR **3/3**, finite-budget regression, typecheck and check-gates **76** pass. No production auth/expiry bypass or assertion/threshold reduction. **Full new-source sweep remains NOT_RUN locally and requires GitHub.** W2/W3 queued UI work remains gated on full LC-U1 acceptance.

The CI4 and earlier paragraphs below are historical scoped proofs, not CI5 whole-unit acceptance.

Author source **`5012ce963db5d88dfd0a9101c4738926c682d218`**, trunk **`f8f01b74`** merged. Latest: [CI4-DELIVERY.md](CI4-DELIVERY.md). Final-source local Console focus **3/3** and Studio main focus **1/1** both exit 0. Signed negative requests reach the unchanged 403 checks, logout/reauth fences pass, replay remains same-key/same-body/one-effect, and native calendar + save/reload + Taipei PG timestamp pass. Node **510/510**, types, check-gates (73 modes) and original privacy guard exit 0. All original assertions/thresholds remain. **E3 for these selected local MOCK/REAL_PG checks only**; full Console/Studio cases and Linux CI remain pending. W2-U2/U3 remain registered, **not started**, until full LC-U1 green and approved base.

The CI3/CI2/D3/CI1 and LC-B7 sections below are historical checkpoints, not current-source runtime claims. Current acceptance/rerun requirements are in CI4-DELIVERY; no whole-unit completion is claimed.

Latest CI2 follow-up: [CI2-DELIVERY.md](CI2-DELIVERY.md). Trunk `71235fc4` is merged; original TCV09 passes after removing the iframe/CSP exception. Console nil receipts, hidden Account sign-out, alert ambiguity and missing sweep CommentStream are source-fixed. Node **496/496**, types and check-gates pass. Local focused attempts were canceled before PG while waiting for the shared lock and remain **NOT_RUN**. New-source GitHub browser execution is pending; the older failure/ruling records below are historical.

Latest D3 follow-up: [D3-DELIVERY.md](D3-DELIVERY.md) records the shared Claims/Console Go/SQL enum, third mode picker, zh-TW/en/ja host prompts, red→green parity and **477/477** Node result. It merges integration `8c3851e4` and requires the additional GitHub `--browser-e2e` real-click matrix. The source hashes there bind the final D3 commit; the older SHA and CI receipts below remain historical.

Historical CI1 source **`c743b465472a657f7652cc80934055d9e1b9577d`** included trunk **`685d465c`**, the Next proxy repair, settings scene selector, updated leaf/title drivers and CI display/serial-fixture configuration. Local Node **465/465**, scoped **25/25**, both typechecks and check-gates exited 0. TCV09 was RED at that checkpoint; the later CI2 non-iframe repair resolved it without weakening the guard. Historical attribution: [CI-37443892071.md](CI-37443892071.md).

The earlier LC-B7 alignment and initial candidate receipts below remain historical evidence. No current browser PASS is claimed.

- Branch: `unit/lc-u1-shell`; previous LC-B7 alignment source: `df8c6a2e7f3e52b45f23225cb055118d473081a6`.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- Integrated base: `8a08656a46ab98db4497240e1ffc6c27b6e697b8`, including GitHub-only heavy-gate rule `2f596a0c`; merge `d50953a8` preserves both `--browser-live-console` and `--browser-tracking-backfill` and both Node registrations.
- Historical LC-B7 evidence level: **E1 overall with the then-blocking source guard**; **E3 for local regressions and reproduced failures**. Current local/CI boundaries are in CI3-DELIVERY.
- Root runtime model/effort: NOT_EXPOSED. Child transport/browser/safety roles requested `gpt-6.1-sol`, high; actual runtime telemetry NOT_EXPOSED. Raw author reports are retained as reports, not independent acceptance.

## Implemented

| Area | Candidate behavior | Boundary |
| --- | --- | --- |
| W0/v5 shell | Sessions, new `/studio/console`, and existing claim settings share registry navigation and three locales. No second store switcher in the new workspace. | Inbox navigation/page remains LC-U2-owned; no dead link was added. |
| Three phases | Server lifecycle selects the single primary start/end/copy action. Copy uses **planning** version, not lifecycle version. A1/A5/A7 accept `KEYWORD_QTY_CONTAINS`. | UI explicitly says these commands do not start/stop the Meta broadcast. Combined runtime acceptance requires LC-B7 on the CI integration branch. |
| Products | Offer switch preserves the existing `max_quantity_per_claim`; local recommendation uses `post_comment:false`; stock uses sellable delta and CAS. Both `inventory:write` and `inventory:live_adjust` enable stock controls independently of `live:manage`. | LC-B7 enforces reason, delta and reserved/allocated/unavailable bounds. No inventory grant keeps stock disabled and the server refusal is preserved. |
| Status/polling | Five-second visible-only, non-overlapping reads; collected revenue and order amounts are separate; IG totals and other unavailable comment counts show a dash, not zero. | A1 business acceptance in this browser fixture is MOCK; actual Go/PG acceptance belongs to LC-B7. |
| Session results/copy | A5 results retain currencies and sandbox money separately; both list and console copy entry points use keyed commands. | A5 reads may be refused without the backend-required permissions; unavailable totals are not fabricated. |
| Media | Verified numeric Page/post link opens Facebook externally with `noopener noreferrer`; no iframe or CSP widening. Honest IG no-embed notice remains. | Access depends on the Facebook account/post visibility. Playback is not certified; test external traffic is MOCK. |
| Command safety | Opaque receipt fence survives reload **and reauthentication**; in-memory replay stays bound to the original session. Delayed copy completion is fenced after navigation/unmount. Transient list/console reads do not lose UNKNOWN retry. | Durable recovery needs authoritative reconciliation; the UI does not guess that a request failed or silently generate a new key. |

No authored changes to product Go, SQL, migrations, dependency lockfiles, `packages/ui`, or frozen `tests/ui/**`. Go changes are confined to the explicitly MOCK browser fixture. See [REVIEW-NOTES.md](REVIEW-NOTES.md) for the independent findings and bounded repairs.

## LC-B7 alignment — 2026-10-06 18:04 ruling

- A1 GET and inventory POST were already in the BFF allowlist. New tests call the **actual BFF route**: A1's real enum previously produced 503; it now returns 200. The inventory POST is forwarded for a store listing containing only the narrow inventory grant, with origin/CSRF/key protections and backend `403` / `422 below_reserved` preserved.
- Replaced the incorrect bare `CONTAINS` value with `KEYWORD_QTY_CONTAINS` throughout the console/A5/A7 parser and fixture. The old wire value is rejected.
- IG has no Graph summary field: its count presentation stays unavailable, including a regression against an accidental numeric observation. FB observed counts retain their distinct label.
- Independent review found two additional blockers for legal large scenes: controls were capped at 100 instead of the backend's 200, and the BFF's 256 KiB limit rejected a legal 200-offer Unicode response. Both have failing-before/passing-after tests; A1 alone now permits 512 KiB, with 201-offer and oversized-response negatives retained.
- CI browser assertions now exercise narrow stock adjustment without `live:manage`, disabled lifecycle/offer/recommend controls, >1000 delta refusal, persisted stock after reload, `below_reserved`, no-inventory disabled controls and a supplemental BFF 403/no-mutation authority test. IG lanes use unavailable totals. This is a contract correction, not a waived permission negative.
- Independent read-only reviewer closed both fixture findings (authorization error classification and independent permission coverage). No browser runtime or screenshot acceptance is claimed.

| Current-source command | Exit / result | Evidence |
| --- | --- | --- |
| `node --experimental-transform-types --test tests/admin/live-workspace.test.ts tests/admin/live-console-model.test.ts tests/admin/live-console-bff.test.ts` | 0; 23/23 | `lc-b7-green.log` |
| `bash scripts/dev/test-node.sh` | 0; 462/462 | `lc-b7-test-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `lc-b7-tsc.log` |
| `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module ESNext --moduleResolution bundler --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/live-console.spec.ts` | 0 | `lc-b7-spec-tsc-bundler.log` |
| `bash scripts/dev/check-gates.sh` | 0; 72 registered modes | `lc-b7-check-gates.log` |
| `gofmt -l tests/foundation/browser_live_console_test.go`; `git diff --check` | 0; no output | Author command receipts |

Red evidence: `lc-b7-red.log` (exit 1, including actual A1 BFF 503), `lc-b7-offer-cap-red-valid.log` (exit 1 at the old cap), `lc-b7-large-a1-red.log` (exit 1, 503 instead of 200). The earlier `lc-b7-offer-cap-red.log` lacked a cache header; it is a fixture setup failure, not cap proof. Two initial standalone spec typecheck commands used incomplete/wrong flags (`lc-b7-spec-tsc.log`, `lc-b7-spec-tsc-green.log`, both exit 1); the bundler/type-root command above is the valid passing check.

NOT_RUN locally: every browser/visual/click gate, Go browser-fixture compilation/runtime, LC-B7 PG acceptance, screenshots, LIVE/SANDBOX. The optional pinned R04 media binary suite is explicitly NOT_RUN by `test-node.sh` because its binary is unset. Source is pinned by `df8c6a2e`; subsequent delivery commits change evidence only.

## CI gates

**Run on GitHub only. The integrator pushes and dispatches; this unit did not push or run the workflow.** Each row is a separate `test-local.sh` mode:

| Exact mode | Purpose | Current result |
| --- | --- | --- |
| `--browser-live-console` | 11 real-click cases: 1586×992 / 390×844 × en/zh-TW/zh-CN, lifecycle/copy/offer/stock/recommend, receipt replay, reauthentication, actual SPA navigation, totals, canonical match mode, IG unknown total, missing A1, narrow-stock success and no-stock denial | NOT_RUN; rerun on LC-U1 + LC-B7 trunk |
| `--browser-studio-ui` | Existing draft/media/leave guards and changed session list | NOT_RUN |
| `--browser-studio-bff` | Shared Studio BFF method/body/auth boundaries | NOT_RUN |
| `--browser-live-claims` | Existing claim/source/settings writes and navigation | NOT_RUN |
| `--browser-admin-shell` | Route registry, permissions, sidebar/breadcrumb parity | NOT_RUN |
| `--browser-visual-lint` | Frozen detector; compare new failures with integration baseline | NOT_RUN |
| `--browser-click-sweep` | Existing routes and navigation/action regression | NOT_RUN |

Workflow `modes` input:

```json
["--browser-live-console","--browser-studio-ui","--browser-studio-bff","--browser-live-claims","--browser-admin-shell","--browser-visual-lint","--browser-click-sweep"]
```

`release-gate.sh --list` resolves all seven `B-browser-*` entries, including the new formal console mode. No hidden alias was introduced.

The new console fixture writes screenshots, Playwright output and redacted `mock-receipts.json` to `output/ci-gates/live-console-*` when `CI=true`, so the existing workflow upload includes them. It uses real signed MOCK OIDC, Next session/CSRF and PG `platform.WithScope`; **console business responses are MOCK** and cannot accept LC-B1/LC-B7 SQL or Meta. Trace is off; cookie assertion diagnostics retain names, not values. Other modes retain their existing artifact paths; please retain their screenshot directories too if they are needed for independent review.

## Initial-candidate lightweight checks (historical)

All below ran against source `b6ccbfe2`; following evidence/document-only commits must not be represented as changed application source.

| Command | Exit | Evidence |
| --- | --- | --- |
| `node --experimental-transform-types --test tests/admin/live-workspace.test.ts tests/admin/live-console-model.test.ts tests/admin/live-console-bff.test.ts` | 0, **20/20** | `ci-candidate-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `ci-candidate-tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0, **72 registered modes** | `ci-candidate-check-gates.log` |
| `bash -n scripts/dev/test-local.sh` and `bash -n scripts/dev/test-node.sh` | 0 | Merge-resolution tool receipt; no browser run |
| `git diff --check` | 0 | Author source/evidence preparation |

Evidence staging note: `git diff --cached --check` returns **2** on raw historical log whitespace/CR progress lines. Those logs are deliberately preserved byte-for-byte; the authored Markdown-only check returns 0. This is not a product test result.

Red evidence is retained for new pure models, CSP, completion fencing and reauthentication fence (`red-*.log`, corresponding `green-*.log`). Some early reds are missing-module or unsupported strip-only syntax failures; they are not falsely described as a browser mutation. The real SPA delayed-copy unsafe-callback mutation remains NOT_RUN.

Historical `depmap.sh --check` returned **1** at the pre-CI candidate because integration's generated Go map was stale. At that check, `cmd`, `internal`, `go.mod/go.sum` and the map were unchanged from integration `261f728b`; no unrelated graph rewrite was authored. Final-baseline depmap runtime is NOT_RUN.

## Interrupted local history and remaining work

- `browser-red.log`: old runner interrupted by the prior host termination; no final exit code. **Not acceptance or a valid browser RED.**
- `red-resume-20261006T060525Z/result.tsv`: **137**, canceled while waiting and owning no PG/container. PID/cwd/parent and absence of its exact container were verified; TERM only ran the existing cleanup trap without exiting, so only that owned waiter was stopped. Other locks/PIDs were untouched.
- `console-current-20261006T061501Z/merge.log`: launcher returned **2 before testing** due to merge conflicts; those are now resolved. No current console browser result exists.
- New owner rule: **no further local browser/full-foundation/visual/sweep/batch starts**. The old local wrapper is archived as `historical-local-runner.txt`, not an active runner. No own browser waiters or the two named local PG containers remain.
- **Integration prerequisite satisfied in source:** LC-B7 `585600b7` supplies A1 and narrow stock authorization on trunk `685d465c`, now merged into this unit as `6dd556a5`. Current-source CI is pending; this UI handoff does not certify the backend.
- CI exits, actual screenshots at both sizes/three locales, independent screenshot review, production/SANDBOX/Meta validation: **NOT_RUN**. Existing unrelated integration visual findings must be identified by baseline comparison, not suppressed.
- Helper worktrees and source commits remain available for review; no push, deployment, live credential access, real message, money or production change occurred.
