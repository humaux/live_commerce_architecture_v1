CONT #289 addresses the independent review's P1: privacy safeguards lack causal RED. **Current head is an intentional, batched negative control. DO NOT MERGE.** The component temporarily bypasses export review, omits erasure scope acknowledgement, and ignores the shared unknown-outcome lock. No deployment or production action is authorized.

The original card implements the owner-signed simplified customer search/profile/consent/privacy UI in zh-TW, zh-CN and en, using existing synchronous server DTOs and APIs. Scope, retention facts and safe unknown-result behavior are the restoration target. Current behavior intentionally violates three safeguards solely to obtain CI negative-control evidence.

All existing browser assertions and corrected DTO fixtures remain unchanged. Four added real-click cases independently reach uncertain erasure results and check both commands stay disabled after return/reload, so the earlier export/checkbox mutations cannot mask this oracle. Expect assertion-level RED for PEOPLE-V2-EXPORT-CONFIRM, PEOPLE-V2-ERASE-SCOPE and PEOPLE-V2-UNKNOWN-FENCE-RETURN/RELOAD. Retain failed browser artifact and run/job IDs, then reverse output/people-v2-impl/evidence/cont289-negative-control.patch and make one restoration push. Final acceptance requires that head's own GREEN and independent review.

Historical evidence, not current acceptance: reviewed 94af3eb58 Gates 40778192684 / bundle:07 job 123136059045 and required 123145022098; Foundation 40778192522 / 123132835742 are green. Initial RED 40761011712 / 123079851595 proved six search failures, while ten privacy cases stopped on malformed timestamps and supply no causal privacy evidence.

Required integration merge completed before this relay. Generated browser ownership includes the updated spec; depmap regenerated with no diff. Static node syntax check exits 0. No test, threshold, fixture, baseline, exemption, startup gate-adapter assertion, API, migration, R2 pin, dependency or lockfile is weakened/changed. No component import changes. Local tests/servers NOT_RUN; author E0; static E1; current RED/GREEN PENDING. DELIVERY.md records restoration, click-ledger expectations and original UI-process/copy/parity handoff. Humaux MCP unavailable; no store/canvas/coord writes claimed. Integrator relays CI conclusions in a fresh session per WAIT_RUN.

Outside original PLAN write paths across this PR, within pre-approved customer module:
- apps/admin/app/[locale]/customers/[customer]/page.tsx: bounded existing q/tag return-query context.
- tests/admin/customer-tags.spec.ts: real-click Advanced before existing controls.
- tests/admin/import-wizard.spec.ts: exact name and scope checkbox for existing erasure.
No additional outside-PLAN path in CONT-289. Generated ownership is an assigned path. Integration imports are unchanged from origin/r3/integration.

P2 findings are listed in the PR comment for integrator issue batching: export refusal notice outside modal; page size mislabeled as total. They are not changed in this P1 evidence relay.

🤖 Generated with Codex (dispatched by Claude Code integrator).
