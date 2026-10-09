<!-- Purpose: Bind LC-U1 CI5 failure attribution, finite fixture-lease/status-heading fixes and real-render guards to source evidence.
Depends on: source 975ed53f, trunk 0617bc40, CI 37567262145/head0b23712f, actual gate3 ledger and production expiry guards.
Used by: Integrator full GitHub rerun and independent acceptance; not a claim that new-source sweep already passes. -->
# LC-U1 CI5 — root repair candidate, full sweep pending

- Branch/worktree: `unit/lc-u1-shell`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- Base merge `9432bc19` includes `0617bc40f4a50e84bd57d6eed00e87167582ca9a`.
- UI commit `d088195c`: safe status h1 + real shared-component SSR guards.
- Fixture commit `975ed53f28b15b2bd78ba767c56092236004fc40`: finite sweep-owned leases and DB-free regression. Evidence-only final commit preserves the six source hashes below.
- Runtime model NOT_EXPOSED; two read-only explorers used inherited model, high/medium reasoning. They reviewed evidence/source, not independent runtime.

## What the 96 failures actually show

Run `37567262145`, head `0b23712f18b7fb2056d3a6e333f70d2269c3fdec`; **gate-3** is the current sweep artifact. The first downloaded gate-1 had stale ledger material and was not used for attribution. Current ledger timestamp `2026-10-07T04:40:42.115Z`, SHA256 `a682adc51a4d26cedec5a01b76439d50ecd1e7f9c0c5f661c9e7f0e8cb2fe0d9`.

| Class | Count | Current evidence |
| --- | ---: | --- |
| page-load | 38 | HTTP 200/no h1; r00416/r00418 show the **expired-session shell**, not a blank page or removed ready-state title |
| coverage-missing | 51 | 17 storefront routes × 3 variants aborted downstream |
| sweep-error | 3 | Storefront product-card waits; r00528 displays **shop not open** |
| journey-step | 4 | J1/J2–3/J4/J5 unavailable after loss of authority/serving admission |
| total | 96 | 427 individual control clicks PASS / 25 SKIP / no failed control-click rows |

`nav-orders` clicked successfully at **r00011**. J5's screenshot is the login form before its intended logout action. Sample expired/closed screenshots are committed under `CI5-evidence/`; raw artifacts stay ignored and retained.

The shared shell/header/navigation/CSS blobs are identical between `5035d211`, the failed `0b23712f` and `0617bc40`; the ready header still renders h1, and Orders is still a singleton. The actual lifecycle conflict is reproducible from source: sweep context **85 minutes**, signed fixture `SessionTTL: time.Hour`, synthetic `bhPublish` domain validity **one hour**; run lasted **3852.56 seconds (64.2 minutes)**. Ready pages passed before authority expired. This is deterministic, not flakiness. Production expiry/hidden-private-data behavior must not be bypassed.

## Changes

1. Only this isolated sweep's domain lease is aligned to its context deadline + **one minute**, scoped by tenant/store/exact synthetic origin and requiring **one updated row**. Only its signed MOCK IdP service uses a finite **remaining-budget + one-minute TTL** instead of one hour. Production policy, grants, revocation, 401 handling and dedicated expired-session negatives are unchanged. Context remains 85 minutes; no click/coverage threshold changed.
2. Loading, expired and unavailable shell states now contain one h1. Private children/store context/navigation remain hidden by the same guards. Ready titles/nav rendering is unchanged; no per-page patch or new navigation entry.
3. New actual JSX SSR tests render WorkspaceFrame/AdminPageHeader/Presentation/AppShell with real registry/copy/access/i18n across three locales. Assert one matching shared h1, a **direct single `nav-orders` group button**, no collapsed Orders group, and safe failure-state headings with no buyer/store data leak. In-memory h1 demotion/second Orders route faults both fail the test. The default fixture is not mutated.
4. New Go budget regression runs DB-free and also through both existing sweep/visual modes. The original sweep test and all its assertions still run; regex now includes the added budget test.

Precision: domain cutoff is absolute deadline+grace. `SessionTTL` is computed once at service construction and applied at later issuance, so a late login's DB expiry can be later than that absolute cutoff; it remains finite (≤86-minute duration), and the test stack is bounded/cleaned. The helper model does **not** prove persisted leases or an immortal/renewing session.

SSR precision: test child contains the actual shared AdminPageHeader; this checks the **shared render contract**, not each complete business page's header wiring. React state/path and network-domain banners are fixture-controlled; effects, hydration, visibility and clicks require the existing sweep. Title expectation shares `pageTitle`; existing registry/copy pins remain. No whole-page/browser E4 claim.

## Commands and red/green

| Command | Exit/result | Evidence |
| --- | --- | --- |
| `node --experimental-transform-types --test tests/admin/shell-render.test.ts` before status-heading change | 1; ready title/nav pass, loading h1 absent | `ci5-shell-red.log` |
| Same command on fixed source | 0; 3/3 | `ci5-shell-green.log` |
| `LC_SHELL_RENDER_FAULT=headings node --experimental-transform-types --test tests/admin/shell-render.test.ts` | 1; missing h1 caught | `ci5-heading-calibration-red.log` |
| `LC_SHELL_RENDER_FAULT=nav node --experimental-transform-types --test tests/admin/shell-render.test.ts` | 1; non-singleton/collapsed Orders caught | `ci5-nav-calibration-red.log` |
| From `tests/foundation`: `go test -count=1 click_sweep_budget_test.go` old one-hour helper / fixed helper | 1 → 0 | `ci5-budget-red.log`, `ci5-budget-green.log` |
| `bash scripts/dev/test-node.sh` | 0; 527 pass / 0 fail | `ci5-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `ci5-tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0; 76 modes, header base 0617bc40, both Go tag sets vetted | `ci5-check-gates.log` |
| `gofmt -l` touched Go tests; `bash -n` dev scripts; `git diff --check` | 0 | Author receipts |

The initial SSR setup used native TypeScript 7 without `transpileModule` and failed; retained `ci5-shell-setup-red.log`. Correct harness reuses the already-installed `typescript-api` exactly like attribution SSR; no dependency added. One first wrong-cwd invocation did not run a Node test; it is not counted as behavior red.

Independent source/evidence reviewers found no P0/P1 and confirmed unchanged grants/negative/default gate assertions; their review is not a rerun of full sweep.

## CI gates / NOT_RUN

Integrator should push the delivered SHA and run full `--browser-click-sweep`, `--browser-visual-lint`, `--browser-admin-shell`, `--browser-live-console`, `--browser-studio-ui`, `--browser-live-claims`. Include `--browser-studio-bff` / D3 `--browser-e2e` per standing regression list. **All focus/filter/calibration env vars must be unset.**

NOT_RUN locally on CI5 source: full sweep/visual, browser modes, full foundation, persisted long-duration fixture behavior, Linux UI, provider/SANDBOX/LIVE. No owned server/container/browser launched this turn, no foreign locks/processes touched. No push/deploy/real messages/keys. The 96 NEW FAIL count is pre-fix evidence, **not a proven new-source zero**.

## Source hashes

| File | SHA256 |
| --- | --- |
| WorkspaceFrame.tsx | `f9d8b38e986af439441a50d1f90c21fb7fe2c531a5f4919011785d5542bd7d1d` |
| shell-render.test.ts | `4533f345132862f5b47f7e9e104ca29130d7f246ba49c40728354f365e4f0be6` |
| browser_click_sweep_test.go | `822477ff3ffaac28e9c640d8168c7108fe46ecb8f5636e39a94cc7ae5c19e6ef` |
| click_sweep_budget_test.go | `58ee3a0b0ace26fabaa69c7ded3c0a218e4206d4fdebea86940fb965e0d216e7` |
| test-local.sh | `f10850bf9ace0f3993d00a24d76d000b7635f21c0ee7c0d6b6c288da5c58361e` |
| test-node.sh | `5282a9f1ae308bb959b356506f87f42246e9fb2963161783c0a9fca9320274b2` |

W2-U2 → W2-U3 → W3-U3 remain queued, not started, until full LC-U1 green and approved base.
