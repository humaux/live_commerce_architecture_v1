# LC-U2b inbox page + BuyerPanel delivery

- Branch: `unit/lc-u2b-inbox-page`; worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u2b-inbox-page`.
- Base: `f73405150a1cca3c588d4546f3b5a8e0d1a456a3` (owner-assigned base overrides the older brief reference). Tested source commit: `b272d802b4279bda6c49f86d363aed4d6a14dc07`; final evidence commit is reported in the author handoff. Source is also bound by `source-hashes.txt`.
- Author: Codex-4, GPT-6-based Codex; exact parent model/effort not exposed. No push/merge/deployment.
- Evidence: **E3 / MOCK for 17 focused Node tests; E1 for TypeScript/structure. Overall acceptance BLOCKED. BROWSER / REAL_PG / SANDBOX / LIVE NOT_RUN.**

## Summary

- `apps/admin/app/[locale]/messages/page.tsx:19`, `components/Inbox.tsx:21`: authenticated store entry, messages registry, list filters/cursor paging and memory-only selection. SSR serializes no buyer/thread data.
- `components/InboxThread.tsx:21`: A9 paging, A10 only with reply permission, keyed A11/A12, authoritative generation/window and fixed delivery/errors; published templates and explicit dm_session capability checks.
- `components/BuyerPanel.tsx:17`: standalone store + conversation/bundle + callbacks, independent hide/session fence; orders gated by orders:read; no inferred link CAS version.
- `src/features/messages/privacy.ts:8`, `use-privacy.ts:11`: abort epochs, controller-bound tickets, synchronous hide clearing, terminal navigation revocation; exact immutable receipt on uncertain retry. Normal Refresh retains unknown delivery.
- `lib/inbox-bff.ts:15`, catchall route: precise A8–A14/GET templates, strict method/query/body, real Request stream/auth/CSRF tests, no-store and fixed safe errors. Bounded 1 MiB admits 50 valid Unicode messages and refuses excess.
- `tests/foundation/browser_inbox_ui_test.go:31`, `tests/admin/inbox-ui.spec.ts:157`: registered `--browser-inbox`, signed MOCK OIDC + real Next/Go/PG, real click/PG readback, privacy/cross-store/read-only/window/capability cases and committed-send/lost-ACK same-receipt retry. Japanese INU07 is mandatory, never skipped.

Contract/interface changes: **none**. Additional write scope: one shell-registry expectation update explicitly approved by the owner. No backend Go, migrations, OpenAPI, module or dependency lock changes; Go acceptance test harness is the brief's explicit exception. `playwright.config.ts` gets the necessary suite registration. zh-TW/en/ja unit copy is complete; shared ja route support is blocked below.

## Actual commands and exits

All commands run in the assigned worktree unless stated. Logs are retained under this directory; trailing whitespace in captured logs is normalized only.

| Command | Exit / evidence |
| --- | --- |
| `pnpm install --frozen-lockfile` | 0; installed frozen dependencies, no lock edits |
| `node --test --experimental-strip-types tests/admin/inbox-privacy.test.ts` before implementation | 1; `red.log` |
| `node --test --experimental-strip-types tests/admin/inbox-bff.test.ts` before BFF registration | 1; worker `bff-red.log`, real handler returned 404 |
| Same privacy command before each new fix | 1 each; `retry-red.log`, `calibration-red.log`, `hidden-focus-red.log`, `navigation-red.log`, `ticket-scope-red.log` |
| Same real BFF command before response-budget fix | 1; `page-budget-red.log` (valid Unicode page returned 503) |
| `node --test --experimental-strip-types tests/admin/inbox-privacy.test.ts tests/admin/inbox-copy.test.ts tests/admin/inbox-bff.test.ts` | **0, 17/17**; `green.log` |
| `pnpm --filter @live-commerce/admin typecheck` | **0**; `typecheck.log` |
| `pnpm exec tsc --noEmit --strict --target ES2023 --lib dom,dom.iterable,esnext --module esnext --moduleResolution bundler --skipLibCheck --typeRoots apps/admin/node_modules/@types --types node tests/admin/inbox-ui.spec.ts` | **0**; `spec-typecheck.log` |
| `node --test --experimental-strip-types tests/admin/shell-architecture.test.mjs` | **0, 2/2**; `architecture-green.log`; initial formatter red retained in `gates-red.log`, reused shared amount/Taipei-time helpers |
| `bash scripts/dev/check-headers.sh` | **0**; `headers.log` |
| `git diff --check` | **0**; no whitespace findings |
| `bash scripts/dev/test-node.sh` | **0** at source commit b272d802; `node.log`; initial exit 1 preserved in `node-red.log`. Existing optional R04 binary suite reports NOT_RUN, not PASS |
| `bash scripts/dev/check-gates.sh` | **0**, 78 modes / 1218 foundation names; `check-gates.log`; old registry red preserved in `registry-red.log`. Owner-approved expectation follows required navigation and adds staff denial; no threshold/allowance widened |

Worker-only registration/static checks and historical failure commands are preserved in `browser-CI.md` and `bff-green.log`; their source hashes refer to worker commits, not this final tree. Parent independently reran the integrated focused tests/typechecks/architecture/header checks above. Required browser bug injection -> red -> restored green is **NOT_RUN**, so the new browser gate is structurally registered but not calibrated/accepted.

## BLOCKED / risks

1. **Resolved:** owner explicitly approved `tests/admin/shell-registry.test.ts` scope extension. Old owner/messages absent expectation now asserts owner visible plus orders-only staff invisible/direct route denied. Registry target 10/10, full Node and check-gates green; red logs retained.
2. Shared `packages/i18n` locales and `app/[locale]/layout.tsx` exclude ja. Outside unit write_paths. Japanese strings alone do not satisfy route acceptance: **INU07 will fail its real shared-language/route assertion until the shared owner adds ja**. Normal browser green/calibration cannot be accepted before this fix.
3. Frozen A13 `internal/inbox/read.go:262` still returns empty claims/orders, ordinal 0, no display name/auto-reply; A8/A9/A13 expose no link version for A14. Honest empty/unavailable states are rendered; manual customer-link controls remain disabled with a reason. No guessed version or read-via-write. Backend owner must supply the missing read model/version contract.
4. A8 validates session_id but does not use it in the query; live_comment projection is absent. Bundle-only rows have no usable conversation/source-link URL: read-only bundle panel and explicit unavailable copy-link control. No fabricated deep link or reply.
5. A9 exposes no binding id for capability matching. UI conservatively requires every matching-provider named dm_session row to be ok/review_required; missing/unknown blocks sends, review_required shows the app-role-only badge. Go is authoritative.
6. Browser, Go compilation, PG runtime, visual QA/screenshots and normal/calibration runs are NOT_RUN under the owner RAM rule. CI harness currently targets headed Chromium; WebKit for this new mode is NOT_RUN.

## Independent work and custody

Base for all worktrees: f7340515. No recursive delegation; interfaces frozen before UI/BFF wiring. Workers committed locally; parent applied their patches in its own branch.

| Role / configured model+effort | Worktree / ownership |
| --- | --- |
| Codex-4 / parent exact variant unknown | assigned unit worktree; page, Inbox/Thread/BuyerPanel, feature privacy/copy/styles/routes and final evidence |
| ui_worker / gpt-6.1-sol high | `.worktrees/lc-u2b-bff`, `codex/lc-u2b-bff`; lib/inbox-types/client/bff, catchall, real-handler test; commit b9139ad1 |
| test_worker / gpt-6.1-sol high | `.worktrees/lc-u2b-browser`, `codex/lc-u2b-browser`; Go/spec + mode/runner/workflow/Playwright registrations; commits b396f8da, fc9bd6c3 |
| explorer / gpt-6-luna medium | read-only frozen DTO/ingress audit; no writes |
| platform_explorer / gpt-6.1-sol medium | read-only Go helper/schema/permission audit; no Go execution |
| security_reviewer / gpt-6.1-sol high | read-only privacy review; receipt, hidden-event and terminal navigation findings fixed; final targeted source review found no further confirmed P0/P1. Not runtime acceptance |

Humaux parent task `d378c85f-2030-4047-8dc9-e297bc45bfd4`; charter/fixes/problems stored, canvas updated and code indexed/linked. Independent terminal recheck memory `cd8d0b66-e163-4a57-8612-2a405d62faa1`. Opus/K3 acceptance still required. No active author processes; own transient patch/index fixtures removed; dependencies and worker commits preserved. No SSH, production host, live keys or real customer messages used.

## CI gates (integrator, NOT_RUN)

Fix shared ja locale and frozen backend gaps, pin the integrated SHA, register run IDs/log paths with a timeout and completion notification, then run on GitHub:

```sh
bash scripts/dev/test-node.sh
pnpm --filter @live-commerce/admin typecheck
pnpm run build:admin
bash scripts/dev/check-gates.sh
bash scripts/dev/test-local.sh --browser-inbox
LC_INBOX_CALIBRATION=retain-thread bash scripts/dev/test-local.sh --browser-inbox
bash scripts/dev/test-local.sh --browser-admin-shell
bash scripts/dev/test-local.sh --browser-click-sweep
bash scripts/dev/test-local.sh --browser-visual-lint
```

Dispatch `gates.yml` normal modes on the integrator's branch; dispatch calibration separately with `extra_env='LC_INBOX_CALIBRATION=retain-thread'`. Calibration must fail **INU05 hidden thread retains private DM** after a trusted native hide, not startup/compile/locale failure. Dedicated loopback acceptance + loopback-test flag gate the intentional defect; generic fixture bypass stays off. Preserve normal-green/calibration-red at identical SHA and require restored normal green. Expected artifacts: next/browser logs, click ledger, native visibility record, 1440/390 screenshots and Playwright traces. None of these runtime artifacts exist yet.
