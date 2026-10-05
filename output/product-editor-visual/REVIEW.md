# Independent review record

Base: `0cc63a1d`. Application source: `dcf12327`; final acceptance-test locator: `ece52ead`.

## Source review (non-author)

`/root/pev_review`, security_reviewer, `gpt-6.1-sol`, high reasoning; read-only in this worktree, no write paths and no recursive delegation. Review of the draft found three P2 regressions, all corrected before final acceptance:

1. Selected-row SKU prefix fill must preserve the full matrix ordinal. A model reproduced `SKU1,SKU1,SKU3`; the corrected result is `SKU1,SKU2,SKU3`. Final browser coverage reapplies the same prefix to just the middle row and saves successfully.
2. Tag entry must not clear pending values when the matrix would exceed 100 SKUs. It now checks the remaining axis capacity first; the browser rejects 3×34, preserves the input and existing three rows, then accepts 3×2.
3. A single-line input would remove pasted newlines. Explicit paste normalization preserves separators. The browser pastes actual clipboard text, commits two chips, and removes them with mouse/Space; keyboard removal returns focus to the input.

Final `dcf12327` read-only review: no new P0/P1/P2. The product-document, catalog-v2-write and use-product-document libraries have no diff from the base (exit 0). ProductReadiness preserves the previous callbacks, labels and states while bringing Form back to 794 lines. `git diff --check` and line-count check exit 0. Reviewer did not run builds, browser or PG gates; these are the author's separate recorded real runs.

Humaux review memories: `cf19f766-e7fa-4603-9e13-5b1fb76ebf09`, final `1679b48f-16c8-48b7-82e3-3e7eef3c048c` (Product editor visual dcf12327 final readonly source confirmation).

## Visual review (non-author)

`/root/pev_harness`, explorer, `gpt-6.1-sol`, medium reasoning; read-only screenshots/metrics, no write paths or builds. Reviewed 10 representative baseline images, then 9 final images covering all six locale/width combinations and the four new/existing empty/three-variant states. Also checked all 24 measurements.

No newly confirmed visual blocker. Six desktop spec rows have zero label/control offset; stock controls are 44px and SKU rows 65px across widths. The 640px field cap, compact empty uploader, localized readiness markers, single image requirement and teal editor controls are visible. All 24 document-overflow values are zero.

Limit: screenshot inspection is not a keyboard/scroll reachability test, nor a WCAG certification. Mobile navigation and SKU matrix intentionally scroll inside their own containers. Full-page captures retain the normal fixed save bar at the viewport bottom; that compositing artifact is not evidence of a permanently obstructed field. Functional/nav/focus checks are in the final browser run.

Humaux memory title: Product editor final visual QA ece52ead 2026-10-05.

The same read-only harness reviewer checked the final sweep isolation setting: `LC_SWEEP_WORKERS=1` is supported and serializes tasks only; sampling, request interception, fingerprint checks and assertions are unchanged. Default-worker failure is consistent with another page's Copy changing a global fingerprint during a local-only Remove. Exact request timing is not proven. Serial acceptance does not certify default-worker concurrency. Memory title: Product editor sweep workers1 is assertion-preserving isolation 2026-10-05.

## Root confirmation

Root independently inspected representative desktop/mobile final captures and ran the recorded final gates. Impeccable audit-first guidance was used to remove repeated structure and align controls with existing shell tokens; this is a scoped adaptation, not a new visual language or shell redesign. No third-party UI library was added.
