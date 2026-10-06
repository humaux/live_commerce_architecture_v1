# R6 independent source follow-up

- task_id: `a41f1e9e-e832-4fd5-aa52-c397fad0b358`
- reviewed base/head: `5a895dea2c20b3942abcfb37364eac8999b6edb5`
- author source: `5ef3be52bbab31b09b00c6dd2699c3cc8bb4a262`
- reviewer: `av_ads_settings`, independent READ_ONLY source review; runtime model/reasoning identifier UNKNOWN.
- source worktree: `.worktrees/admin-visual`
- changed paths: this report only; no source/test/writer/receipt edits.
- compared prior research memory: `e6e8776b-2870-497d-be5a-5aa463731694`.

## Original findings

P1 hidden feedback/retry: fixed in source for changed outcomes. Form groups existing message/retry/result DOM under a `tabIndex=-1` feedback ref. The hook scrolls only the owning pane to that wrapper and focuses it with `preventScroll` when message/done ID/recovery state changes. Initial recovery-fence hydration sets the message and recovery state, retriggering this layout effect. Ordinary asynchronous retry clears the old message before awaiting and supplies the subsequent outcome, so a repeated UNKNOWN response reveals again. Existing actual retry predicates and alert/status roles are preserved. Runtime GREEN remains UNKNOWN to reviewer.

P2 observer incremental-entry navigation: fixed in source. The hook retains all ordered section elements scoped to the pane, inspects their rectangles against the pane on each passive scroll/observer notification, and confines its focused-section override to those actual targets. Its frame, listener and observer are cleaned up. No new confirmed P1 was found in the inspected changes.

## Residual P2 — repeated identical synchronous validation does not reveal again

Location: `ProductDocumentForm.tsx:93-97`, `useProductEditorLayout.ts:61-74`; existing writer validation `use-product-document.ts:192-206`.

Sequence: publish without a photograph; the image-required message reveals; navigate back to Basics without editing; publish again. The writer sets the same message string. The feedback key remains identical and the layout effect does not run, leaving the existing notice off the current pane viewport. This also applies to unchanged no-changes/matrix validation. There is a workaround (scroll back down), so P2, not a new receipt/state blocker.

Recommendation: expose a presentation-only reveal operation or attempt counter for submitting/repeating an existing validation outcome. Do not change writer receipts, UNKNOWN retry authority or persistence. Acceptance: real repeated no-photo Publish separated by a nav click must reveal the existing message again.

## Residual verification gaps

The existing Another/reset action removes the focused feedback subtree without explicitly returning focus/scroll to an initial field. This was not introduced by this diff; keyboard reset navigation remains a runtime gap, not an additional release-blocking finding. Long feedback in short/keyboard-reduced viewports, actual screen-reader announcement and asynchronous unmount/remount geometry remain UNKNOWN/NOT_RUN.

## Evidence and commands

All inspection commands ran in `.worktrees/admin-visual`:

- `git show 5a895dea --stat` — exit 0.
- `git show 5ef3be52 -- apps/admin/components/ProductDocumentForm.tsx apps/admin/components/useProductEditorLayout.ts` — exit 0.
- `git show 5a895dea:apps/admin/components/useProductEditorLayout.ts | nl -ba` — exit 0.
- Targeted `git show ... | rg -n ...` and `git show ... | sed -n ...` for Form and writer validation/retry/reset — exit 0.
- `git diff --exit-code 5a895dea^ 5a895dea -- apps/admin/lib/use-product-document.ts apps/admin/lib/product-document.ts apps/admin/lib/use-product-leave-guard.ts` — exit 0, empty: writer, document serialization and leave-guard source unchanged by this commit.

Tool-output evidence: `8bfd93`, `99ee7a`, `edefa9`, `dfb7dd`, `953d78`, `bad760`. Browser, builds, PG and test execution: NOT_RUN by reviewer. No acceptance criteria or test changes. This source review is not sole-author or runtime acceptance.
