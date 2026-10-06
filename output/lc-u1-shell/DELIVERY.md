<!-- Purpose: Hand off the LC-U1 UI candidate and exact GitHub-only acceptance gates without claiming unrun browser results.
Depends on: contracts/live-console-v1.md LC-U1/Amendment 1, source b6ccbfe2, integration 8a08656a and owner rule 2f596a0c.
Used by: The integrator pushing unit/lc-u1-shell and running .github/workflows/gates.yml; independent review. -->
# LC-U1 — candidate ready for CI, not runtime-accepted

- Branch: `unit/lc-u1-shell`; source commit: `b6ccbfe24a453c203d36fd0e04a34105b21399e7`.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- Integrated base: `8a08656a46ab98db4497240e1ffc6c27b6e697b8`, including GitHub-only heavy-gate rule `2f596a0c`; merge `d50953a8` preserves both `--browser-live-console` and `--browser-tracking-backfill` and both Node registrations.
- Evidence level: **E1 overall** (typed/registered candidate). The 20 focused Node assertions have automated evidence; this does not establish browser, SQL, provider or production acceptance.
- Root runtime model/effort: NOT_EXPOSED. Child transport/browser/safety roles requested `gpt-6.1-sol`, high; actual runtime telemetry NOT_EXPOSED. Raw author reports are retained as reports, not independent acceptance.

## Implemented

| Area | Candidate behavior | Boundary |
| --- | --- | --- |
| W0/v5 shell | Sessions, new `/studio/console`, and existing claim settings share registry navigation and three locales. No second store switcher in the new workspace. | Inbox navigation/page remains LC-U2-owned; no dead link was added. |
| Three phases | Server lifecycle selects the single primary start/end/copy action. Copy uses **planning** version, not lifecycle version. | UI explicitly says these commands do not start/stop the Meta broadcast. A1 real read model is still missing. |
| Products | Offer switch preserves the real existing `max_quantity_per_claim`; local recommendation uses `post_comment:false`; stock uses sellable delta and CAS. | Existing `inventory:write` works through the existing endpoint; `inventory:live_adjust`-only users remain disabled with an explanation until LC-B7 lands. No backend permission widening. |
| Status/polling | Five-second visible-only, non-overlapping reads; collected revenue and order amounts are separate; unavailable comment counts show a dash, not zero. | A1 contract acceptance is MOCK, not proof that its backend route exists. |
| Session results/copy | A5 results retain currencies and sandbox money separately; both list and console copy entry points use keyed commands. | A5 reads may be refused without the backend-required permissions; unavailable totals are not fabricated. |
| Media | Optional official public Facebook-post iframe from a validated numeric Page/post pair and a verified source; console-only CSP allowance; honest IG no-embed notice. | Live-video identity selection/playback is not proven. No external Meta request/mutation was used for acceptance. |
| Command safety | Opaque receipt fence survives reload **and reauthentication**; in-memory replay stays bound to the original session. Delayed copy completion is fenced after navigation/unmount. Transient list/console reads do not lose UNKNOWN retry. | Durable recovery needs authoritative reconciliation; the UI does not guess that a request failed or silently generate a new key. |

No authored changes to product Go, SQL, migrations, dependency lockfiles, `packages/ui`, or frozen `tests/ui/**`. Go changes are confined to the explicitly MOCK browser fixture. See [REVIEW-NOTES.md](REVIEW-NOTES.md) for the independent findings and bounded repairs.

## CI gates

**Run on GitHub only. The integrator pushes and dispatches; this unit did not push or run the workflow.** Each row is a separate `test-local.sh` mode:

| Exact mode | Purpose | Current result |
| --- | --- | --- |
| `--browser-live-console` | 11 real-click cases: 1586×992 / 390×844 × en/zh-TW/zh-CN, lifecycle/copy/offer/stock/recommend, receipt replay, reauthentication, actual SPA navigation, totals, missing A1 and narrow-stock denial | NOT_RUN |
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

## Actual lightweight checks

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
- **Backend gaps at the integrated baseline:** A1 console GET is not mounted; `POST /inventory/adjustments` still requires `inventory:write`. A6 is mounted. These are not UI fixes or waived acceptance. Real operator rollout stays blocked until LC-B7 and its tests are integrated.
- CI exits, actual screenshots at both sizes/three locales, independent screenshot review, production/SANDBOX/Meta validation: **NOT_RUN**. Existing unrelated integration visual findings must be identified by baseline comparison, not suppressed.
- Helper worktrees and source commits remain available for review; no push, deployment, live credential access, real message, money or production change occurred.
