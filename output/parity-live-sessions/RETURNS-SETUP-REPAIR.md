# PR365 Returns restricted-session setup repair — first attempt
Date: 2026-11-02. Coord43a7ffec-69e2-4bbd-87ca-d776a3664a55 is claimed by root; this dispatched child did not claim another task or delegate.

## Assignment and scope
Root reviewed the original SOURCE report and froze this first setup repair (attempt1/2). Requested routing: Sol/high, preserving the original source context; actual runtime model ID/reasoning UNKNOWN. Worktree .worktrees/codex-pr365-mock-balance, local branch codex/pr365-mock-balance, incoming base d883d0749605b4c670d1b3ff188e245a0d69c45b. Incoming git status was clean and no author lock was active. This author acquired pr365:author before editing and releases it at handoff. The same root-assigned worktree is used as explicitly instructed; no new worktree, branch or competing writer was introduced.

Allowed/authored files only:
- tests/admin/returns-ui.spec.ts
- output/parity-live-sessions/RETURNS-SETUP-REPAIR.md

Generated browser ownership is intentionally left for root's final generation. No fixture, helper, product, backend, guard, permission, gate, timeout, count budget, retry, dependency or Figma file changed.

## Exact change and cause
Removed only the restricted-member case's redundant await signedLogin(page) at former line291. Its existing awaited asToken(context, restrictedToken) now runs before the first document navigation in that test's fresh context. All other cases retain their original login, and asToken itself remains byte-identical.

The original default-owner login mounted a Dashboard that could issue its deferred finance read during clearCookies/addCookies. Original trace shows the cookie-less finance request returning401 with session/CSRF expiry after addCookies completes; the order document and shell store-list request then have no session. This repair removes that unnecessary mounted owner page from the synthetic restricted-session setup. It does not change the application's authentication behavior.

All returns history, no-action, hostile-write status401/403 and final Go state/money/no-effect assertions are retained exactly. No product permission outcome or actual token validity is inferred from this static edit.

## Original RED and source authority
- Parent-frozen instruction: one setup-line deletion, two named paths, local static checks and local commit only, no push/dispatch/runtime.
- SOURCE report in root checkout: output/integrator/tools/logs/ci-evidence-1238/365-returns-source-0934.md SHA256 e510d8156ab5a9cc52e9913b721300b50663c4c4ad3df51d397c71b29f5b483a.
- Original failure report: output/integrator/tools/logs/ci-evidence-1238/365-returns-failure-0925/report.md SHA256 402d03fc37229e8ad1c0906d91710e20ccf967a349303d46dc0c42e974c06ecc.
- Gates41415863028 / bundle07 job125181194383 / own execution efbf7281e7e4a20dcb2e7b5ebfd37186b6556299, PR head d883d0749605b4c670d1b3ff188e245a0d69c45b.
- Artifact13128827838 original nested trace4-trace.zip SHA256 359d3d469cc501084a494d431551c51de1754bbfd1a13825f5c7680108cfed67. SOURCE audit checked nested CRC and matched12 direct source Git blobs to the own execution tree. Original evidence is unchanged.
- Original case30 failed at merchant-orders visibility after10000ms. Original trace contains22 nonstatic requests, no403/500, no order-read/returns API calls in that failed case. Its intended permission and forged-write assertions were not reached. This existing automated RED precedes the edit; another PR/Meta result is not evidence for this repair.

## Static verification
| Check | Exit / result |
|---|---|
|Incoming spec equals git show d883:tests/admin/returns-ui.spec.ts|0, exact bytes|
|Unique restricted-case anchor and deletion length27bytes|0|
|Reinsert only the deleted line and compare to incoming spec|0, all other19831bytes preserved|
|node --experimental-strip-types --check tests/admin/returns-ui.spec.ts|0; parse only, spec not executed|
|git diff --check on the two allowed paths|0|
|git diff --numstat -- tests/admin/returns-ui.spec.ts|0 added,1 deleted|

Incoming spec SHA25614bd880a95d1fa9898fefd158f9a66d96cbae1b1d97016a110585d6d3135d67f,19858bytes. Repaired spec SHA256fac82c19c92d2aacc42a7ac71357dcd66b101408c8475e5305aa001fa495e980,19831bytes. Exact-byte validation proves every other byte, including all negative assertions and the token helper, was preserved. ponytail and the loaded development rule were applied: use the existing helper and delete only the redundant setup.

## Acceptance and remaining risk
E1 structure/static repair only. GREEN is NOT_RUN. Root must generate/check official browser ownership, publish under its own control, bind the next CI receipts to their own execution SHA, and require:
1. Actual restricted session reaches the real order read and CLOSED return history; it must not stop at an expired shell.
2. All original absent-write-controls and forged-write rejection assertions execute unchanged, followed by original Go no-extra-RMA/restock/money/cancel checks.
3. Registered Returns browser mode passes and root performs independent acceptance. If authenticated restricted access then fails, preserve that new real product RED and investigate; do not weaken the shell or permission guard.

No local Go/PG/browser/provider/CI/server, push, remote dispatch, merge or deployment. SANDBOX/LIVE NOT_RUN. No current-head runtime permission success is claimed. The known risk is that intervention efficacy has not yet been exercised; this is the first root-approved directed setup attempt, not a completed acceptance.

## Handoff and rollback boundary
Only these two paths are committed locally. Root receives the exact local commit and final clean status separately; root alone handles generated ownership and any push/CI. No background job was started and no runtime fixture/container/port requires cleanup. Original SHA d883 is the rollback reference; revert this isolated local commit if root rejects it, preserving unrelated work. No destructive rollback was performed.
