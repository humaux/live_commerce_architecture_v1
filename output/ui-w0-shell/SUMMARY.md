# ui-w0-shell handoff — implementation delivered, acceptance NOT GREEN

2026-10-02. Only `unit/ui-w0-shell`, base `b4223c8448d4fabf5f6f5f7de19e84b0d99bad40`, worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/ui-w0-shell` was edited. No push, merge, deploy, Go/SQL, production host or credential access. Integrator must not treat this as release approval.

## Decisions

| Decision | Status | Evidence / boundary |
| --- | --- | --- |
| 1 Shell | IMPLEMENTED; new shell MOCK browser PASS | 220px dark rail, ten registered groups in prescribed order (messages hidden without routes), bottom Settings, working store/language/help/account, no search/task/bell placeholders. Drawer below 1024px; extra 375px tested. |
| 2 Registry | IMPLEMENTED; node PASS | 23 page paths mapped bidirectionally; domain route modules generate navigation, title, breadcrumbs, UI 403 and gates. Permission UX is fail-closed; Go remains authority. `studio/claims` stays `nav:false`: store/scene context is necessary. `orders/new` and `products/import` spec coverage here is shell smoke, not business form acceptance. |
| 3 Legacy unchanged | IMPLEMENTED; HUMAN APPROVAL PENDING | 46 pre-switch captures (23 routes × 1586×992 / 390×844), committed under `tests/admin/baselines/w0/`. Page bodies retained, with shell wrapper / metadata changes only. Public invite metadata retains no-referrer and noindex/nofollow. Captures use isolated MOCK data; no claim of owner baseline sign-off. |
| 4 packages/ui | IMPLEMENTED | Only shell + CSS Module + tokens, no speculative table/form library. Removed old shell CSS, retained domain-body CSS. Comp 01 controls shell; scoped DESIGN override records dark/orange/1024px rules. |
| 5 packages/format | IMPLEMENTED; existing helper tests PASS | Existing money/Taipei implementations moved, not duplicated; old admin modules re-export. TWD uses NT$ integer display. Legacy offenders outside package are explicit shrinking allowances, not silently rewritten. |
| 6 Copy | IMPLEMENTED; node PASS | `typeof en` typed zh-CN / zh-TW / en shell copy. Missing/extra keys, labels and registry consistency tested. |

## Commits

All commits include `Co-Authored-By: Codex <noreply@openai.com>`.

| SHA | Change |
| --- | --- |
| 3a2f3d2 | Preserve legacy route screenshot baseline and isolated capture harness |
| d6f4ad2 | Correct signed-out reset/signup baseline captures |
| b2dd0e3 | Move existing amount/time implementations to packages/format |
| 3f28c26 | Registry shell, tokens, role guards and architecture/browser gates |
| 947474c | Independent visual fixes; navigation locator adaptations; correct design browser dispatch |
| a4e0cec | Public registry titles and explicit design-test preflight; intermediate invite metadata build failed |
| b89bd75 | Resolve invite metadata export conflict while preserving privacy metadata |
| 9eca57b | Correct remaining Studio return-navigation locator |
| 9826388 | Use registry IDs for catalog navigation; catalog mode subsequently passed |
| 37b8eb9 | Complete required info semantic token |
| 2d2f3fd | Sign-in helpers wait for permitted integration navigation, not unrelated Orders |
| 52486bd | Explicit 401 recovery, localized sign-in entry, fail-closed domain mounting, 44px target and browser regression |

Evidence/SUMMARY commit is the commit containing this file; use `git log -1 -- output/ui-w0-shell/SUMMARY.md` after handoff.

## Gates and exit codes

| Gate / exact command | Exit | Status / evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | 284 pass, 0 fail, `test-node-handoff.log` at 52486bd |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | 0 | `tsc-handoff.log` at 52486bd |
| `bash scripts/dev/check-gates.sh` | 0 | 56 modes, `check-gates-handoff.log` at 52486bd |
| G-UI1 registry | 0 | Included above; four registry node tests. Initial missing-module red exit 1 in `../ui-w0-registry-red.log`, green in `../ui-w0-registry-green.log`. |
| G-UI2 `bash scripts/dev/test-local.sh --browser-admin-shell` | 0 | `shell-handoff.log` at 52486bd; 24 viewport/locale cases, 13 destinations clicked in each, geometry/hit testing/long store/drawer/roles/store switch. 9 public title checks plus explicit 401 recovery/no-domain-content/44px sign-in link. |
| G-UI3 formatting / copy | 0 | Compiler-AST gate, explicit immutable basis + shrinking allowance. Initial red 1, green 0 in `../ui-w0-architecture-{red,green}.log`. Existing helper tests retained. |
| G-UI4 shell accessibility | 0 | New shell mode: rail/topbar axe serious/critical = 0, desktop actual Tab sequence, mobile focus trap/Escape/return, ≥44px targets. This is shell chrome, not full legacy-page accessibility certification. |
| G-UI5 architecture | 0 | Compiler API checks fetch boundaries, deep cross-feature imports, cycles, line limits; positive and negative tests. Allowances cannot grow against frozen basis or HEAD/HEAD^ removal history. New dependencies documented. |
| G-UI6 approved visual direction | PROVIDED | Existing owner-approved comp 01, no new visual-world redesign. This does not substitute for final K3. |
| G-UI7 old modes | NOT GREEN | Individual runs and unresolved scope conflicts below. No assertions removed/weakened. |
| Required independent K3 visual QA | NOT_RUN / BLOCKED | Repository K3 runner reads protected credentials and writes outside this worktree; both prohibited in this assignment. Integrator must run separately. |
| Human legacy baseline approval | NOT_RUN | Committed images are available for review; no approval inferred. |

Final application source is 52486bd. New shell, node, tsc and architecture gates were all rerun there, exit 0. Session recovery regression: `session-expiry-red.log` exit 1 (missing dedicated expired state), `session-expiry-green.log` exit 0, followed by final `shell-handoff.log` exit 0 including the 44px target assertion. Browser evidence tiers below use real packaged Next and local Go/PG where the existing wrapper does; external Meta/payments/logistics remain MOCK or explicitly SKIPPED, never LIVE.

### G-UI7 ledger

All commands are `bash scripts/dev/test-local.sh --<mode>`. First-round timestamps/exit codes are in `regression-exits.tsv`; logs are `<mode>.log`. First round used 3f28c26. Final/recheck files retain later attempts rather than overwrite failures.

Six modes have recorded passing runs; five remain red. These are pinned revision-specific results, not a claim that all 11 passed on final 52486bd: G-UI7 was not repeated in full after the dedicated 401 fix. Do not approve release while blocked gates remain.

| Mode | Latest exit | Result / stop line |
| --- | --- | --- |
| browser-admin-legacy | 1 | BLOCKED: old ledger fixture has no identity `/api/stores`; entry fixture omits role/permissions. Shell fails closed. Additional old flat-rail assumptions require adjudication, not assertion deletion. |
| browser-catalog-core | 0 | PASS `browser-catalog-core-accepted.log` at 9826388: real Go/PG, en/zh-TW, desktop/390px. Final fix only replaced old translated nav text with registry IDs. First-round status/search timing failure retained as a flake risk, not hidden. |
| browser-catalog-media | 0 | PASS first round; original business assertions retained. |
| browser-promotions | 0 | PASS first round. |
| browser-ops-polish | 1 | BLOCKED: fixture session lacks live:read, new route guard correctly prevents Studio body; cannot change Go fixture or permission assertions within navigation-only scope. |
| browser-customers-billing | 1 | Final 11 pass/1 fail: test expects old body-specific forbidden copy; shell correctly shows centralized 403 with no customer table. Changing permission assertion is beyond navigation-locator authorization. See `browser-customers-billing-final.log`. |
| browser-studio-ui | 1 | `browser-studio-ui-accepted.log`: real signed UI/BFF/Go/PG/MOCK-worker main chain passes; 4/5 cases pass. Last failure expects old Studio-specific expired-session wording. New shell had also collapsed 401 into generic failure; fixed separately with explicit sign-in recovery and red/green shell test. Domain assertion remains unchanged and requires integrator adjudication. |
| browser-merchant-orders-ui | 0 | PASS first round. |
| browser-cvs | 0 | PASS first round, including WebKit; sandbox SKIP because keys are outside authorized scope. |
| browser-meta-connect | 0 | PASS `browser-meta-connect-accepted.log` at 2d2f3fd, both independent gate and merchant connect suite. Fixture legitimately has integration permissions, not orders:read: helper now waits for authorized Settings navigation, no fixture grants or business assertion changes. Meta external = MOCK. |
| browser-design | 1 | Correct existing `TestBrowserStoreDesign` run, 4 pass/4 fail in `browser-design-accepted.log`. Sign-in now works. Remaining journeys intentionally navigate to Customers to test unsaved-change guards, but their Go fixture lacks customers:read. Changing that journey/URL assertions or Go grants exceeds navigation-locator-only scope. Initial wrong dispatch's exit 0 is NOT design coverage. |

Intermediate `*-recheck.log` / `recheck-exits.tsv` at a4e0cec all returned 1 during build (duplicate invite metadata export); browser cases NOT_RUN for that attempt. b89bd75 fixes it; these logs are not counted as executed business failures.

### Remaining decisions for integrator

1. Authorize/adapt legacy Go and mock fixture session metadata to the real role/permission contract, including design's explicit Customers navigation. Meta's minimal-permission fixture was already handled by a navigation-only helper correction; do not add broad grants there. Do not loosen the new shell guard to satisfy fixtures.
2. Resolve centralized shell 403 versus legacy body-copy assertions; preserve absence-of-data assertions and add/retain explicit BFF denial tests where required. Existing task allows only nav-locator changes, so these assertions were left untouched.
3. Catalog final mode passes, but preserve its first-run status/search timing evidence as a flake risk; no domain-body changes made here.
4. Run required K3 review and obtain human baseline approval. Supplemental review below is not a replacement.

## Visual evidence and independent review

`shell-{locale}-{width}x{height}.png`: 24 captures (zh-CN, zh-TW, en × 1366×768, 1586×992, 2000×1100, 1024×768, 768×1024, 390×844, 375×812, 360×740).
`drawer-{locale}-{width}x{height}.png`: 12 mobile/tablet drawer captures. `session-expired-en.png`: extra 360×740 expired-session recovery evidence (37 final screenshots total). `browser-results.json` is the machine-readable outcome. `failure.png` / `failure.txt` are preserved old failure evidence, not final acceptance screenshots.

Impeccable audit-first was used to keep domain bodies unchanged, constrain shell-only redesign and review actual screenshots. Detector ran once: `detector.json` = `[]`. Fresh independent read-only reviewer inspected all 36 captures against comp 01: no P0/P1, two P2 issues (design-rule conflict documentation, typographic close icon) were fixed in 947474c. Reviewer reopened all 12 drawer captures and issued a scoped ship verdict for those two fixes only. Humaux records: `1c6a0031-3d18-4ad3-b6d3-23ce706c7a4c`, `25e025c8-8bb7-4990-9ad5-e0008ffa2c08`. Not K3; not whole-project release certification.

Additional narrow independent review of 52486bd and `session-expired-en.png`: P0/P1 = 0, domain remains unmounted on 401, local locale sign-in path, 44px target verified. Humaux `7278eac7-07ca-41d3-a334-d82807960f10`. No additional detector run; still not K3.

## Traceability / boundaries

- Primary writer: Codex runtime (actual model not exposed; no override), unit/ui-w0-shell, base above. Only this worktree's TS/JS/CSS/docs/tests/package metadata and generated evidence. No Go/SQL changes. Source indexed in Humaux; why-links attached to WorkspaceFrame, canOpen and AppShell. `.mjs` gate entity was not resolved by code index, so no link claimed for it.
- Read-only helpers: r5_gate_paths (gpt-6-luna/medium), r5_image_audit (gpt-6-luna/medium), r5_visual_review (gpt-6.1-sol/medium), w0_finish_review (gpt-6.1-sol/high, fresh no-history review). No helper wrote production code; no recursive delegation.
- New shell role result is a rendered UI 403 state (HTTP page response can be 200), not proof of server HTTP 403. MOCK shell tests explicitly say backend authorization NOT_PROVEN_BY_THIS_MOCK.
- Browser traces/failure contexts remain under this worktree's `output/playwright/`; they are not bulk committed because they can contain synthetic session fixture material. Exact evidence directories are in each log.
- NOT_RUN: production/LIVE writes; provider sandbox requiring credentials; K3; human baseline approval; full release-gate beyond requested modes; existing R04 LiveKit binary branch when its environment is unset. No evidence here certifies those.
- Cleanup verified after final run: no running process referencing this worktree; Docker only showed pre-existing `humaux-thread-qdrant` and `humaux-thread-pg`. A concurrently observed `lc-focused-51006` belonged to another unit's `test-focused.sh`; it was left untouched and later exited. All own browser/Next/fake-API workers exited through runner cleanup. Failure logs/traces remain; no foreign directories or containers removed.
- Evidence log scan found no live/test Stripe key pattern, Meta EAA token pattern, Authorization Bearer value or session-cookie value; generated synthetic fixture code stays explicitly MOCK. No private config was read. Test logs and output are scoped to this worktree.
