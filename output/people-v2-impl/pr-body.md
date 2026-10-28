Implements the owner-signed simplified customer UI: concise search/view/return, responsive customer facts and orders, purpose/source/date consent, review before export, and exact-name plus scope-checkbox erasure. All displayed customer values use the existing server DTOs. Three-locale screen copy lives in packages/i18n. Existing optional tools remain under Advanced.

Privacy commands keep the existing synchronous API. Unconfirmed transport results take the read-only submitted/pending/return path with a session-scoped opaque safety marker; reload cannot silently replay or unlock a command. No backend queue, API, contract, migration, pin, lockfile, baseline or gate exemption changed.

RED: head 8e19c18721bca3f778a7ff7683afcfd5e5b2f708; Gates 40761011712 / job 123079851595 (bundle:07), artifact 12853485502. Six new signed-search assertions failed on the missing placeholder; 12 existing browser cases passed. Ten new detail cases did not reach confirmation assertions because the MOCK order timestamps lacked six fractional digits. Corrected those timestamps and the blocked-error envelope to the unchanged DTO; retained every assertion and added visible-history/title checks. Full receipt: output/people-v2-impl/red.log.

Static validation: strict admin tsc with temporary source aliases, node --check, check_packet.py, ownership and depmap generators exit 0. Ordinary local tsc lacks installed shared-package links; no production config change. Local tests/servers NOT_RUN by owner instruction. Own implementation CI/GREEN and independent acceptance PENDING. Author assessment E0; static structure E1; old-head causal search RED E3. UI click ledgers and raw native density measurements are emitted by CI. No merge/auto-merge/deployment.

Outside original PLAN write paths, within the pre-approved customer module:
- apps/admin/app/[locale]/customers/[customer]/page.tsx: accept only the existing bounded q/tag grammar so Back to results persists after refresh; unknown/repeated queries still refuse.
- tests/admin/customer-tags.spec.ts: real-click Advanced before existing controls, including after reload.
- tests/admin/import-wizard.spec.ts: exercise exact customer name plus scope checkbox for existing imported-customer erasure.

Generated browser ownership is included after merging origin/r3/integration. depmap.sh produced no diff. Existing compiling harnesses updated in the same push. DELIVERY.md contains the required parity checklist, UI PROCESS sections, three-language copy, risk and NOT_RUN lists. Unknown outcomes stay locked in that authenticated tab until a definite outcome or new session; there is no server request-status endpoint to invent an automatic unlock from. Rollback: revert the implementation commit; no database or production write.

CI startup repair (attempt 1): GitHub reported DIRTY/CONFLICTING with no checks on implementation 7eed1ab3c. Merged current integration dcecd6220, reconciled only the i18n header while preserving both feature exports, and regenerated ownership. Strict mapped tsc and index syntax checks exit 0. Other imported code is unchanged from integration. A single integration repair push allows the PR workflows to start.

🤖 Generated with Codex (dispatched by Claude Code integrator).
