<!-- Purpose: Bind the final two runtime regression fixes to genuine red/green browser evidence. -->
<!-- Depends on: exact source commits, immutable isolated Playwright logs and the original Go/PG assertions. -->
<!-- Used by: admin-visual independent review and final gate summary. -->
# Readiness and disclosure regression

- Valid Studio RED: ac6824f0, wrapper `../gates-20261005T104821Z/browser-studio-ui.log`, exit1. `red-playwright.log` shows enabled New scene while its real listGET is deliberately held. The test's earlier19c locator failure is not this red.
- GREEN: f0b93ca6d56d01c68e8787dcf692de7318dfe0f5, wrapper `../gates-20261005T105006Z/browser-studio-ui.log`, exit0,6/6. Original lostACK/same-key/other-login/backend-effect assertions remain.
- MOU RED:0e2b5c55, wrapper `../gates-20261005T101241Z/browser-merchant-orders-ui.log`, exit1,9/10. Fault-injection selection attempted a hidden filter; existing conditional disclosure click repairs the test path.
- MOU GREEN:f0b93ca6, wrapper `../gates-20261005T105006Z/browser-merchant-orders-ui.log`, exit0,10/10. No privacy/fault/recovery assertion removed or relaxed.
- Independent source review: Humaux8e91f326-55ac-4b60-87df-7ad4b0b9913f, no new P0/P1. Session boundary comparison, permission checks, pending keys and uncertain retries unchanged; author implemented only UI readiness/guard/reason and the actual filter interaction.
- Sources after0e2: only Studio.tsx and these two browser specs. Local isolated signed MOCK IdP, Go/PG and MOCK worker; no real broadcast or provider operation.

The original trace remains in the local Playwright evidence directory. It is not copied into Git, because only the red PNG and failure/result logs are needed here; no production credentials were used.
