Implements the owner-signed StatePanel `1429:34298` and InlineError `1429:34299`, with one primary action, 44px targets, UNKNOWN re-reading the same operation and non-disclosing forbidden copy.

PR #274 review continuation: incoming RED `ce10fcee` was invalid because Gates `40750174036` / build `123035247199` failed TS2322 before browser acceptance; Foundation `40750174047` / `123034488051` stopped at the component-fetch architecture gate. No assertion-level RED is claimed for that head.

This corrected RED batch removes unreachable implementation from the legacy-output negative controls while keeping valid exports/types. Gallery fixture reads now reuse the existing admin `readJSON` helper with a bounded GET; the SSR harness resolves its import explicitly and the synthetic browser fixture supplies the helper's required no-store response header. Existing assertions stay intact and in order. Generated browser ownership is refreshed.

Acceptance remains CI-only. The actual forbidden caller and StatePanel/InlineError behavioral assertions must fail first; then the prepared `output/ds-states/implementation.patch` restores the implementation and safe localized caller body in one push. This RED head is intentionally not mergeable. Real click acceptance covers all recovery controls in zh-TW/zh-CN/en at 1440/390, visible effects, refresh persistence, one same-operation GET, no writes/replay and 44px hit rectangles.

Original write paths: `packages/ui/src/ds/{States.tsx,index.ts,states.module.css}`, `tests/ui/ds-rework.test.mjs`. Integrator explicitly approved additional paths: `apps/admin/components/UiGallery*.tsx`, `packages/i18n/src/{index,state-recovery}.ts`, the two browser click files, generated browser ownership and `output/ds-states/**`. This continuation only changes UiGalleryRecovery, ds-states-clicks, ownership and output/ds-states outside the original paths. No further approval requested; no other paths changed in this batch. Initial dirty/untracked output/ds-states-v2 and output/ux-states-ref remain untouched and excluded.

E1 static checks: node --check on both browser click modules and ds-rework.test.mjs; git diff --check; ownership generation, exit 0. tsc NOT_RUN (absent). No local tests/servers/builds or dependency installation. Corrected assertion-level RED, GREEN, independent review, LIVE/provider/PG/WebKit/Figma acceptance remain PENDING/NOT_RUN. New workflow run IDs are recorded in the handoff and PR comment; integrator resumes this lane after CI. No merge, auto-merge, deployment or gate/baseline/exemption changes.

Delivery and original CI failure evidence: `output/ds-states/DELIVERY.md`, `output/ds-states/red.log`. Existing UI process, copy/parity tables and click-ledger contract remain in `output/ds-states-v2/DELIVERY.md`.

🤖 Generated with Codex (dispatched by Claude Code integrator).
