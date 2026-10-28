PEOPLE-V2-IMPL prepares real-click acceptance for the owner-signed simplified customer UI. This is the single tests-only RED push; product implementation follows only after CI proves the new behavioral assertions fail.

The new browser spec is appended to the existing CB11 harness and covers the main screen in zh-TW, zh-CN and en at desktop/mobile widths; search/view/return; export review; name-plus-checkbox erasure; synchronous success/refusal and refresh; uncertain-result read-only return with no automatic replay. Existing tests and assertions remain unchanged. Scoped MOCK DTOs and transport faults are fixture preparation, not backend/production acceptance.

Validation: node --check exits 0; browser ownership regenerated after merging current origin/r3/integration (base bf161bac96252c35337e07713c08a275d48e3e17). Local tests and servers NOT_RUN by owner instruction. Causal RED PENDING; GREEN NOT_RUN; author assessment E0. Current run/job receipts will be recorded in output/people-v2-impl/red.log and the session handoff.

Scope: exact planned paths are in output/people-v2-impl/PLAN.md. Added tests/admin/people-v2.spec.ts; appended registration in tests/foundation/browser_customers_billing_test.go; generated output/ci-select-backend-browser/browser-ownership.json; card evidence in output/people-v2-impl/. Files outside the frozen plan: none. No contract, backend package, migration, lockfile, startup assertions, baseline or exemption changes.

Risk/continuation: CD6 remains synchronous. Signed UNKNOWN/read-only frames describe unconfirmed client outcomes; they must not be implemented as invented server queue state. All UI implementation, runtime density/parity and independent acceptance are pending. No merge or auto-merge is authorized. Rollback: revert this card's commit; no database or production state changed.

🤖 Generated with Codex (dispatched by Claude Code integrator).
