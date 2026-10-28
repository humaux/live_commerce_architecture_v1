Closes #245 (CA-AUTH-COPY part A). Password forms stay inert before hydration and now explain loading and JavaScript requirements in zh-CN, zh-TW and en. A CSS-delayed DS link becomes visible after ten seconds and performs a native reload of the current resource path/query even when page chunks fail. Hydration removes the status and recovery link; positioning outside flow avoids moving form fields.

Adds browser coverage for all three locales: no-JS inert forms and native reload, aborted chunks followed by click-to-recover, and absence of recovery UI after normal hydration. Pre-card tests remain unchanged.

FIX 1/2: WebKit run `40619949465`, job `122622312675`, failed because Playwright's text matcher excludes NOSCRIPT roots. The same job's screenshot already visibly renders the complete explanation; this is not evidence of a hidden product message. The pinned [Playwright 1.63.0 implementation](https://github.com/microsoft/playwright/blob/v1.63.0/packages/injected/src/selectorUtils.ts#L57-L60) confirms the matcher behavior. Retarget the card's newly added full-text assertion to the rendered DS status child of noscript and add its explicit visibility assertion; retain the existing exact visible-text, inert controls and native reload assertions. Add explanation-absence checks after normal hydration. No product fallback rewrite is warranted by this evidence. No engine skip, weaker expected string or timing change. Receipts: output/ca-auth-copy/fix1-red.log and fix1-webkit-no-js.png. Renewed Chromium/WebKit CI and independent review remain PENDING.

RED: tests-only `781887326d96ae0e291e3ce3ab2a87a2e28636f9`, Gates run `40612180005`, job `122576061200`. The new noscript assertion failed at password-auth.spec.ts:189 because the element did not exist; twenty tests did not run under serial mode. Evidence: output/ca-auth-copy/red.log and red-playwright.log. GREEN implementation CI: pending. Independent review: pending with integrator. Author PASS is E0.

Local verification: permitted static `python3 scripts/check_packet.py`, exit 0 (architecture structure only); its generated timestamp was restored. No local tests, servers, builds, browser, PG or containers. DS and functional acceptance require this PR's CI. LIVE/production: NOT_RUN.

FIX 1/2 local verification: NOT_RUN; only source, official logs/artifacts and git diff inspected. Exactly one push is planned for this correction, followed by Gates/Foundation run registration and handoff to the integrator; no long CI polling.

Scope: PasswordAuth.tsx, entry-copy.ts, password-auth.spec.ts and output/ca-auth-copy/**. No persistent out-of-card file changes. No dependency-map regeneration needed (no Go dependency change). No gate/baseline/dispatcher changes. Existing direct-fetch helper is unchanged and outside this recovery UI change. Native reload retains the resource path/query; SSR cannot read an invite URL fragment.

BLOCKED.md remains historical evidence of the original unsplit scope; the revised card excludes mail_unavailable/OIDC capability work. Humaux interfaces were unavailable; card/GitHub evidence is provided for integrator recording. No merge or auto-merge. Rollback: revert this card's commits.

🤖 Generated with Codex (dispatched by Claude Code integrator).
