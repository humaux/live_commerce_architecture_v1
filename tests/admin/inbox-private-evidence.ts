// Purpose: suppress private-page diagnostics from both fallback ARIA capture and matcher-provided snapshots.
// Depends on: isolated worker/test-auto fixtures, fallback snapshot guard and installed artifact teardown order.
// Used by: credential-only inbox spec; ordinary specs keep the unextended test and their normal diagnostics.
// Invariants: I11 no credential DOM capture; error identities/message/stack/status survive; environment is restored.
import { test as base } from "@playwright/test";

/** Define the isolated worker guard without changing environment during module import or test collection. */
export const test = base.extend<{ _inboxPrivateMatcherEvidence: void }, { _inboxPrivateEvidence: void }>({
  // Normal test-auto teardown runs before built-in all-hooks-included _setupArtifacts writes error-context.md.
  // The worker flag only suppresses fallback capture; matcherResult.ariaSnapshot bypasses that guard.
  _inboxPrivateMatcherEvidence: [
    async ({}, use, testInfo) => {
      try {
        await use();
      } finally {
        for (const error of testInfo.errors) delete (error as { errorContext?: string }).errorContext;
      }
    },
    { scope: "test", auto: true },
  ],
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
