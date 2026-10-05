<!-- Purpose: Record the LC-U1 dependency and write-scope checkpoint before application edits.
Depends on: contracts/live-console-v1.md section 16 and source at 3a7181155a01194608abe4faf16c16a97daf153c.
Used by: The integrator allocating LC-U1 frontend paths and the LC-B7/backend dependency owners. -->
# LC-U1 preflight — not a delivery

- Branch: `unit/lc-u1-shell`; base: `3a7181155a01194608abe4faf16c16a97daf153c`.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- Status: dependency/scope checkpoint; no application implementation or acceptance claim.
- Predecessor: admin-visual author delivery `a0af86262101242de464ab4b1d412bf1fe6735ba`; tested source `6126273bae5e1addbe4fc6a8afa2656074523b08`. Its global visual gate remains exit 1 for three out-of-scope storefront R9 findings; no waiver is implied.

## Confirmed interfaces and boundaries

| Item | Current source | Consequence |
| --- | --- | --- |
| A5 results/copy | `internal/httpapi/live_flow.go`, `internal/live/results.go`, `internal/live/copy.go` | Backend exists. Admin BFF does not yet allow these paths; there is no session-results/copy UI client. |
| A7 lifecycle | `internal/httpapi/live_lifecycle.go:27` | Backend exists. Its CAS is lifecycle version, distinct from the planning version used by session copy. Do not substitute one for the other. |
| A1 console read model | No mounted console route found in the current backend route/symbol audit | LC-B7 dependency; aggregate statuses/statistics cannot be fabricated from other states. |
| A6 recommend | No mounted recommend route found; timeline recording helper is not an HTTP implementation | Keep dependent recommendation writes blocked; do not expose a false working control. |
| Narrow live stock edit | `internal/httpapi/handler.go:186` still requires `inventory:write`; `internal/inventory/inventory.go` lacks the contract's narrow live-adjust bounds | Do not replace `inventory:live_adjust` with broad inventory permission. Backend owner must wire the bounded path. |
| Frontend integration | `apps/admin/app/api/stores/[store]/[...resource]/route.ts:68–78` has both Studio and method-specific allowlists | A1/A5/A6/A7 require scoped BFF integration, not a bypass. |
| Navigation | Only `/studio` and hidden `/studio/claims` in `src/features/live/routes.ts`; `/studio/settings` and `/inbox` pages absent | Do not add dead navigation. Inbox remains LC-U2-owned. |

Two independent read-only code explorations covered backend contracts and frontend seams. Their findings are static evidence, not successful HTTP/PG execution. Root checked the current branch, contract unit row, BFF patterns and mounted A5/A7 declarations. No live provider was called.

## Required scope ruling

The LC-U1 row restricts writes to feature/live, four named components and the route aggregator. A working implementation additionally needs:

1. Necessary Next page entry points, locale navigation/breadcrumb copy, and the existing Studio BFF allowlist. New typed clients/models can remain in the allowed feature directory where practical.
2. `tests/admin/**`, a minimal Go browser fixture under `tests/**` (test code only), Playwright registration, `scripts/dev/test-local.sh`, and `docs/delivery/GATES.md` for the explicitly requested formal browser mode. No frozen `tests/ui/**` detector/click-sweep changes.
3. Separate backend owner delivery of A1, A6 and the narrow live-stock path. No product Go/SQL changes are requested for this UI unit.

After frontend scope confirmation, A5 session results/copy, navigation and typed UI work can proceed independently. A1/A6/stock-dependent work can use the frozen contract in explicitly labelled MOCK acceptance but cannot claim real backend integration before the missing routes land.

## Checks actually run

- `git status --short`: exit 0; clean before this checkpoint document.
- `git rev-parse HEAD` and `git rev-parse r3/integration`: exit 0; both the base above.
- `pnpm install --offline --frozen-lockfile`: exit 0; reused 49 packages, downloaded 0; lockfile unchanged.
- Focused source/route inspection: static evidence only.

## NOT_RUN

LC-U1 red/green tests, application implementation, actual-page baseline audit, `--browser-live-console`, affected legacy browser modes, visual lint, typecheck and final static gates. No second test-local mode, server, provider action, push or deployment was started. Approved v5 and W0 conventions were inspected; no alternative visual direction or new UI dependency was introduced.
