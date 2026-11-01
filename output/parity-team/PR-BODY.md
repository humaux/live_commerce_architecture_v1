Implements ROUND 3 of PARITY-TEAM on the current integration base: signed staff/member and pending tabs, five described roles, reachable role/removal/withdrawal review and results, and read-only unknown-outcome reconciliation. Existing DTOs/commands only. Invitation entry is built, but submit displays the signed unavailable state and never sends mail. Continue remains disabled for admin/unchanged roles; no admin confirmation/result is built.

Authority/report: `output/parity-team/RULING.md`; complete desktop/mobile frame→CI screenshot→signed PNG mapping and three-locale table are in `output/parity-team/DELIVERY.md` (historical sections explicitly marked).

RED E3 (failure only): head `89452970349f92219e1b1217868611f1a73057fe`, Gates `41279053547`, browser job `124769729983`, missing signed 員工名單 at tests/admin/parity-team.mjs:171. Build and Foundation `41279053499` / `124767027990` passed. All original RED assertions stay verbatim and in order; new ruling tests only add coverage.

Validation E1: tsc strict, node --check, packet structure check, depmap regeneration and browser ownership generation/--check exit 0. No local tests or servers. Current GREEN, click/persistence/density parity, independent review and SANDBOX/LIVE are NOT_RUN/pending PR CI. Own PASS is E0.

**Acceptance conflict retained:** the untouched suite still asserts successful invitation writes/acceptance and unreachable role-confirm-admin/role-done-admin. Those assertions conflict with this explicit ruling. No skip, fixture/oracle edit, threshold change or test weakening was made; integrator must reconcile deferred scope. Shared-shell module-nav expectations are also untouched. This PR does not claim all frames passed.

### DEFERRED

| State | Desktop / mobile | Reason |
|---|---|---|
| invite-sent | `1468:75217` / `1468:75260` | staff invite outbox: separate backend card |
| invalid-email | `1468:75278` / `1468:75326` | staff invite outbox: separate backend card |
| already-member | `1468:75349` / `1468:75393` | staff invite outbox: separate backend card |
| invite-limit | `1468:75412` / `1468:75456` | staff invite outbox: separate backend card |
| owner-disabled | `1468:75475` / `1468:75519` | staff invite outbox: separate backend card |
| reauth | `1468:75538` / `1468:75582` | staff invite outbox: separate backend card |
| invite-checking | `1468:76405` / `1468:76453` | staff invite outbox: separate backend card |
| checking-unresolved | `1468:76689` / `1468:76734` | staff invite outbox: separate backend card |
| link | `1468:76880` / `1468:76892` | staff invite outbox: separate backend card |
| signin | `1468:76904` / `1468:76925` | staff invite outbox: separate backend card |
| signup | `1468:76946` / `1468:76968` | staff invite outbox: separate backend card |
| verify-signin | `1468:76990` / `1468:77006` | staff invite outbox: separate backend card |
| verify-signup | `1468:77022` / `1468:77038` | staff invite outbox: separate backend card |
| accept | `1468:77054` / `1468:77066` | staff invite outbox: separate backend card |
| accept-checking | `1468:77078` / `1468:77097` | staff invite outbox: separate backend card |
| joined | `1468:77116` / `1468:77134` | staff invite outbox: separate backend card |
| expired | `1468:77152` / `1468:77170` | staff invite outbox: separate backend card |
| revoked | `1468:77188` / `1468:77206` | staff invite outbox: separate backend card |
| used | `1468:77224` / `1468:77242` | staff invite outbox: separate backend card |
| wrong-account | `1468:77260` / `1468:77278` | staff invite outbox: separate backend card |
| invitee-member | `1468:77296` / `1468:77314` | staff invite outbox: separate backend card |
| invitee-off | `1468:77332` / `1468:77350` | staff invite outbox: separate backend card |
| invite-sent-owner | `1468:77368` / `1468:77411` | staff invite outbox: separate backend card |
| invite-sent-live | `1468:77496` / `1468:77539` | staff invite outbox: separate backend card |
| invite-sent-fulfilment | `1468:77624` / `1468:77667` | staff invite outbox: separate backend card |
| invite-sent-viewer | `1468:77752` / `1468:77795` | staff invite outbox: separate backend card |
| mail-failed send/history journey (unavailable content implemented) | `1468:75601` / `1468:75647` | staff invite outbox: separate backend card |
| role-confirm-admin | `1468:78293` / `1468:78333` | signed frame disables Continue |
| role-done-admin | `1468:78348` / `1468:78392` | signed frame disables Continue |
| reauth-signin | `1468:96313` / `1468:96358` | staff invite outbox: separate backend card |
| reauth-verify | `1468:96378` / `1468:96420` | staff invite outbox: separate backend card |

Invitation entry role variants and unavailable content are implemented. Historical pending invitations/withdrawals remain reachable without issuing mail. No internal/identity, migration or contracts/merchant-identity change.

### Scope disclosure

`packages/i18n/src/team-parity.ts` is outside dedicated team surface paths: required three-locale shared copy, integrator approval requested in review. `apps/admin/components/team.css` is an in-module style addition covered by scope pre-approval. `output/ci-select-backend-browser/browser-ownership.json` is generated and explicitly authorized. Existing additive shell-test registration from RED remains; no new shared-shell source change. Depmap regenerated, no diff. No contract/OpenAPI/go.mod/lockfile/baseline/dispatcher/R2-pin or other-app/backend changes.

Humaux shared store/canvas/coord and code index/link NOT_RUN: tools unavailable in this runtime. Current CI run IDs go in a PR comment after push; no evidence-only commit and no production action. Integrator owns queue monitoring/re-dispatch.

🤖 Generated with Codex (dispatched by Claude Code integrator).
