// Purpose: suppress credential-bearing Playwright ARIA error snapshots only during the sensitive worker lifetime.
// Depends on: Playwright worker-auto fixtures and the installed PLAYWRIGHT_NO_COPY_PROMPT snapshot guard.
// Used by: credential-only inbox spec; ordinary specs keep the unextended test and their normal diagnostics.
// Invariants: I11 no credential DOM capture; assertion errors are untouched and prior environment is restored.
import { test as base } from "@playwright/test";

/** Define the isolated worker guard without changing environment during module import or test collection. */
export const test = base.extend<{}, { _inboxPrivateEvidence: void }>({
  _inboxPrivateEvidence: [
    async ({}, use) => {
      const previous = process.env.PLAYWRIGHT_NO_COPY_PROMPT;
      process.env.PLAYWRIGHT_NO_COPY_PROMPT = "1";
      try {
        await use();
      } finally {
        if (previous === undefined) delete process.env.PLAYWRIGHT_NO_COPY_PROMPT;
        else process.env.PLAYWRIGHT_NO_COPY_PROMPT = previous;
      }
    },
    { scope: "worker", auto: true },
  ],
});
