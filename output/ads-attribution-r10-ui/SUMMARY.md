# R10 UI and browser specification slice

task_id: ea5a1fe3-669d-4a4a-8052-a35d5988c350
base_commit: 0d8f939204da1a8498c65938e198b030411d454e
branch: unit/ads-attribution-r10-ui
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution-r10-ui
role: UI author / browser-spec author; independent acceptance remains root-owned.
model: GPT-6 family per runtime instructions; exact model identifier UNKNOWN.
reasoning: UNKNOWN (inherited, not exposed).
skill: frontend-architect (existing Next.js architecture, display-only extraction, strict checks).

## Changes and audit

Write paths: apps/admin/components/Attribution.tsx; apps/admin/lib/attribution-format.ts;
tests/admin/attribution-format.test.ts; tests/admin/attribution.spec.ts;
tests/admin/attribution-checkout.spec.ts; output/ads-attribution-r10-ui/.

Audit of existing UI found the existing Intl number helper used default precision for both
draft/session ROAS; it displayed 2.5× and up to three decimals. Existing report sections,
tables, source boundaries, actions, auth, API DTO and styles were retained. Shared formatROAS
now displays exactly two decimals and preserves localized unknown for null/nonfinite values.
The session ratio still uses the existing server-supplied net/spend facts; no new money source.

AT5 keeps the real storefront checkout path and PG provenance checks owned by Go. Its six
UNPAID bank transfers must show zero collected orders/net and zero pending COD orders/amount.
Only the explicitly incorrect previous six-paid-orders expectation and related ledger text
were replaced; existing date, draft, source, Meta separation, reload and layout assertions stay.

AT7/AT9 retains all original assertions and adds exact independent strings for three locales:
insufficient audience, not_authorized, America/Los_Angeles account timezone, provisional marker,
promoted-post source row and collected-order label. Real selectOption/reload operations visit
each new backend state and restore the primary session. State screenshots enter the same
SHA256 manifest; the new state operations enter the existing click ledger. No DOM mutations,
response interception, direct API shortcut or synthetic completed orders were introduced.

## Frozen runner controls (root approved)

Report API JSON remains unchanged. LC_ATTRIBUTION_FIXTURE must additionally provide:
state_sessions={insufficient:<UUID>,not_authorized:<UUID>}; meta_account_timezone=
America/Los_Angeles; provisional=true; primary live_audience.status=available.
All three session IDs must be distinct. The insufficient session has NULL views,
peak_concurrent and total_view_time_ms, and empty demographic/region arrays. The unauthorized
session displays no metrics/tables. The primary draft contains its promoted-post row.
LC_ATTRIBUTION_CHECKOUT_FIXTURE keeps existing fields with orders=0/net_minor=0 and adds
pending_orders=0/pending_minor=0. Root supplies genuine PG fixtures; browser cannot skip absent
metadata. Six real checkout provenance rows remain verified outside report totals.

## Actual commands and exits

All commands ran in the worktree above unless noted. Logs copied to the permanent main-checkout
evidence directory /Volumes/data/live_commerce_architecture_v1/output/ads-attribution-r10-ui/.

| Command | Exit | Evidence / scope |
| --- | --- | --- |
| node --test --experimental-strip-types tests/admin/attribution-format.test.ts (before formatter exists) | 1 | roas-red.log; missing-module RED retained |
| node --test --experimental-strip-types tests/admin/attribution-format.test.ts (mutation: min0/max3) | 1 | roas-mutation-red.log; 3 PASS / 3 FAIL: 2.5× != 2.50× in all locales |
| node --test --experimental-strip-types tests/admin/attribution-format.test.ts (fixed min2/max2) | 0 | roas-green.log; 6 PASS / 0 FAIL / 0 SKIP |
| pnpm install --offline --frozen-lockfile | 0 | install.log; existing dependencies only; lockfile unchanged |
| pnpm exec prettier --write apps/admin/lib/attribution-format.ts tests/admin/attribution-format.test.ts tests/admin/attribution.spec.ts tests/admin/attribution-checkout.spec.ts | 0 overall enclosing shell | format.log; formatting step before admin tsc |
| pnpm --filter @live-commerce/admin exec tsc --noEmit --incremental false | 0 | typecheck-admin.log; strict admin check |
| node --test --experimental-strip-types tests/admin/attribution-format.test.ts tests/admin/attribution.test.ts tests/admin/attribution-audience.test.ts | 0 | node-green.log; 16 PASS / 0 FAIL / 0 SKIP; pure/MOCK only |
| pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module esnext --moduleResolution bundler --allowImportingTsExtensions tests/admin/attribution.spec.ts tests/admin/attribution-checkout.spec.ts tests/admin/attribution-format.test.ts | 1 | typecheck-spec.log; root has no direct @types/node; configuration failure retained |
| pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module esnext --moduleResolution bundler --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/attribution.spec.ts tests/admin/attribution-checkout.spec.ts tests/admin/attribution-format.test.ts | 0 | typecheck-spec-node-types.log, typecheck-spec-final.log; existing admin Node types, no dependency changes |
| pnpm exec prettier --check apps/admin/lib/attribution-format.ts tests/admin/attribution-format.test.ts tests/admin/attribution.spec.ts tests/admin/attribution-checkout.spec.ts | 1 then 0 after formatting | format-check.log then format-final-check.log |
| pnpm exec prettier --write tests/admin/attribution.spec.ts tests/admin/attribution-checkout.spec.ts | 0 | format-final.log |
| git diff --check | 0 | diff-check.log |

## Evidence boundary and remaining work

Implementation/typecheck/pure Node slice only. REAL_PG, BROWSER, focused/full gates, SANDBOX,
LIVE, production deploy and Meta mutation: NOT_RUN. Root explicitly withheld runtime gates
until the second independent review batch. Therefore AT4/AT5/AT9 acceptance and final merge
remain UNKNOWN/PENDING; this slice cannot close those gates. Root must integrate actual state
fixtures and paid-only SQL, inspect the diff, then independently execute existing formal gates.
No new processes/containers/ports/PG fixtures were started by this agent.

Humaux fix memory: a3dca1b5-0a08-4ed7-8185-b35e186fd184; title:
R10 UI shared ROAS formatter and mandatory three-locale state assertions.
Five modified/new source files indexed; formatROAS and audienceStateClicks linked to the fix.
Canvas r10_ui_author records compile-only handoff and pending root acceptance.
