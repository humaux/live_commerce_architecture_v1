PEOPLE-V2 implements the owner-signed customer search/profile/consent/privacy flow in zh-TW, zh-CN and en. Export requires review, erasure requires the exact displayed name plus scope acknowledgement, and an uncertain outcome keeps both commands locked across return and reload. Existing synchronous server APIs and retention rules stay authoritative.

CONT289 resolves the independent review's P1 evidence gap with a batched causal CI negative control, then restores all three safeguards. CustomerDetail is identical to the reviewed `94af3eb58` source; all PEOPLE assertions and corrected DTO fixtures remain unchanged. Four independent uncertainty cases remain enabled. Original assertion excerpts and hashes are in `output/people-v2-impl/evidence/relay7-red-assertions.log`.

The same head's OC gate exposed a setup race: the signup helper awaited the verify response but returned before PasswordAuth's redirect, allowing createStore's navigation to abort. The shared helper now awaits the expected locale landing page with the default load state. No sleeps, retries, timeout increases or assertion changes. This is the first targeted OC fix. Scope follows the current card's same-admin preapproval; no auth product/backend/API changes.

Validation and acceptance:
- Causal privacy RED: Gates `40803107511`, bundle07 job `123216904686`, head `d409e4094b797aa86987f49923f3e64ce6bf2f99`, synthetic merge `23f0dac876c0296755cfee8522ec8dce4c0245c6`: missing export review, scope unchecked yet enabled, independent RETURN/RELOAD commands unlocked. Migration-import fails the same intentional checkbox mutation.
- All four failed jobs read directly. OC RED: bundle08 `123216904501`, ERR_ABORTED at createStore; receipts `123235265909` and required `123235420197` fail downstream. All three failed browser artifacts freshly downloaded and original logs independently read.
- Browser ownership regenerated (53 modes); depmap regenerated with no diff. Both browser spec syntax checks and diff whitespace check exit 0 (E1).
- Own restoration-head Gates/Foundation and independent delta review: PENDING. Final SHA/run IDs in the PR comment. Historical reviewed-head GREEN is not current acceptance. Author E0; causal CI RED E3.
- NOT_RUN: local tests/builds/servers/browser, tsc (compiler/admin dependencies absent), independent runtime reproduction, LIVE/production. No merge or auto-merge.

Outside original PLAN write paths, within the card's same-admin preapproval:
- `apps/admin/app/[locale]/customers/[customer]/page.tsx`: bounded q/tag return-query context.
- `tests/admin/customer-tags.spec.ts`: real-click Advanced before existing controls.
- `tests/admin/import-wizard.spec.ts`: exact name plus scope checkbox before existing erasure.
- `tests/admin/oc/reauth.spec.ts`: await signup navigation before the next setup navigation (CONT289).
Generated `output/ci-select-backend-browser/browser-ownership.json` is assigned; integration imports are unchanged from origin/r3/integration. No API, migration, R2 pin, contract, lockfile, baseline, exemption or startup gate-adapter assertion changes. No component import changes.

P2 findings are listed in the PR comment for integrator issue batching: definite export refusal outside the still-open modal; page size mislabeled as total. Neither is changed in this P1 continuation.

Delivery includes original UI-process, three-language copy, fact parity and click-ledger handoff. No new design/Figma writes. Rollback: revert the restoration commit; never merge/deploy the negative-control head. Integrator relays own workflow conclusions in a fresh session; no polling beyond ten minutes. No production action authorized.

🤖 Generated with Codex (dispatched by Claude Code integrator).
