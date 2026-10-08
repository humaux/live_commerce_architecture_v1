<!-- Purpose: W5 visible readiness current evidence.
Depends on: original main raw SHA256=ff1dc9adcd0701c046fa7db051dbcd71a9c19528fcbdd99d29de3c09055c7899.
Used by: integrator normal/calibration CI. -->
<!-- Purpose: narrow independent source review of the W5 signed-helper readiness wait correction and its AST regression.
Depends on: parent HEAD019a0995, actual WorkspaceFrame/AppShell CSS and the hash-bound readiness-test worker source below.
Used by: parent task a83bad3d-2b4d-4b4a-b598-85f9e4a99cea; source review only, not browser/authentication or CI certification. -->

No concrete P0/P1 found in this one-line wait correction or the 54-line regression source. Reviewed 2026-10-07 by `/root/w1_ci_diagnosis`, configured same-family gpt-6.1-sol/high; exact runtime model is not exposed. Parent worktree HEAD: `019a09952272c189aacb73b3411ba2a0369154af`.

`tests/admin/import-wizard.spec.ts:46` now waits on `shell-store-selector` after the identity-service sign-in click. `WorkspaceFrame.tsx:291-317` renders that exact select in the topbar once a current-key store exists; `:71-72`, `:121-136`, `:242-244` derive it from successful authenticated workspace reads and discard it on error/logout. The old `workspace-sign-out` target (`:351-364`) sits inside a `<details>` without `open`, so a default-visible wait can stall despite successful login. The selector is the native transparent overlay of the visible store brand (`AppShell.module.css:169-182`); it is not inside the closed account disclosure. Its mobile store layout is retained (`:297-303`). No shell behavior was changed.

The inspected parent diff changes only this wait target. The real sign-in click, Secure/HttpOnly `__Host-` cookie assertion (`spec:47`), import-page navigation and wizard-visible assertion (`:49`) remain. No force, attached-only wait, sleep, auth bypass, case removal or calibration change was introduced by this line.

The worker regression extracts/transpiles the actual `signed` function AST, not a rewritten helper. It asserts sign-in precedes the default-visible selector wait, keeps cookie/wizard assertion calls, covers representative zh-TW390/en1440 paths, and rejects either insecure or non-HttpOnly cookies. Its browser/page, fixture-control callback and expect surface are explicitly MOCK; the test hardcodes the source-confirmed closed-account model. It does not measure actual browser layout or prove the identity callback. At inspection time the test was in clean worker commit `bd1b0efe89b50157cc03b18e129e4afe2151b2bc`; it was not yet present at the parent test path.

Existing worker evidence was read, not rerun: `red-status.json` is finished exit1 with the named closed-details wait failure (0PASS/1FAIL); `green-status.json` is finished exit0 (1PASS/0FAIL). These are author MOCK receipts, not independent reviewer runtime evidence. No Node, Go, PG, Next, browser, authentication, CI or calibration execution occurred in this review. The merged W6 stack and user-reported CI dispatches are outside this narrow review and confer no browser acceptance here. Reviewer has no persistent process; parent fresh checks and final manifest must bind the imported test and current spec bytes.

| Source | SHA256 |
|---|---|
| Parent tests/admin/import-wizard.spec.ts | 669f2b3807a66a0d884eada96427a79a76032b6ab3becaebd7e45dbd984b1d21 |
| Parent apps/admin/components/WorkspaceFrame.tsx | e45fc2a08d4ebaf3f154088702a11e9c27d2ce44a99c8f3c8a335c03fca41aa1 |
| Parent packages/ui/src/AppShell.module.css | 3336d9a4f4e78afbbe3516dfb6fcbaa18d1c786a4351ab59ba379c27c9afffe5 |
| Parent apps/admin/src/shell/api.ts | 52322912d67e306b919b79024c1f1e2e00c82a0482882a61d86c9ec0c1f29e98 |
| .worktrees/w5-u1-readiness-tests/tests/admin/import-readiness.test.mjs | 27be8aa53e9e92e093e3b7eece6660368883dfded9ada83f8dd366238b08d853 |
