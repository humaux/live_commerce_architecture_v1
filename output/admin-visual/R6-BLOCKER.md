<!-- Purpose: Preserve historical R6/R9 diagnosis and its approved resolution without relabelling red evidence. -->
<!-- Depends on: immutable source-bound captures and harness-owner commit82eebefb. -->
<!-- Used by: admin-visual integrator review and historical failure provenance. -->
# Historical R6: raw bounds versus clipped visible controls

Current disposition: **RESOLVED BY HARNESS OWNER**, not waived. Commit82eebefb (imported via3ed634be) measures ancestor-clipped visible rectangles for R6/R9, with original thresholds and pure/canary coverage. Admin visual blocking/R4 became0 on0e2b5c55; whole command still exited1 for three out-of-scope storefront R9 findings. Later exact-source receipts live in SUMMARY.md.

Integrator ruling rechecked2026-10-05: unit/visual-lint-clip points to82eebefb50261a129231ce086f6b96ce5b9d75f2; merge3ed634be40d1ba2bc6b6db16bb6e38eab7802151 has that commit as its second parent and is an ancestor of the current sourcef0b93ca6. `git diff --exit-code 3ed634be HEAD -- tests/ui` returned0: this UI unit did not author detector changes. The unchanged runner was already rerun onf0b93ca6 (105217Z corpus): admin blocking0/R40, whole exit1 with3 storefront-home R9 cases. Do not treat the old raw-rectangle ruling as a waiver for those different remaining cases. No additional rebase while the pinned final gates are active.

Everything below is preserved historical diagnosis, including the then-pending ruling; it is not the current blocker status.

## Rebased R9 readback (2026-10-05)

On integration baseline65902dbe the frozen collector adds R9. `8ee1127c` completed294/294 captures (`output/ui-visual-audit/20261005T083757Z/`, exit1): the same3 R6 instances,2 R9 instances, plus46 genuinely clipped locale-select values. The real locale defect was separately fixed (max-width88→96); its new native-width assertion first failed with56px text /54px available, then `--browser-admin-shell` passed at a4b456b4. No detector edit.

R9 reports the Chinese new-product 分类/分類 button at x335…383 on390px, giving7px raw right margin. A separate **real-browser read-only measurement** at a4b456b4 records the owning scrollport at x16…374 and the actual visible button at x335…374: **16px visible right margin** in both zh-CN and zh-TW. See `output/playwright/catalog-core/20261005T085621.428648000/playwright.log`, lines prefixed `EDITOR_VISIBLE_CLIP_DIAGNOSTIC`. The companion checks never replace the frozen gate, mutate fixture DOM/CSS or change thresholds. The full product-editor command was red for an unrelated obsolete image-checklist expectation; these emitted measurements are not presented as an overall PASS.

The rebased new-product route has **no R6 text-overlap instances** in any captured locale/width. Remaining R6 is the existing English product editor and still needs the harness owner's clipping-semantics adjudication. Exact R6 native hit-testing on that fixture remains NOT_RUN; R9 now has actual scrollport measurements. No arbitrary layout spacing, hidden controls or sticky classification was introduced to evade either rule.

The following section preserves the earlier e856 source-bound investigation unchanged.

Source `e85654d00f684a855f7600d5af94a3ac29c67f46`; unchanged visual runner from `70d9d1d2`. Command `LC_TEST_LOCK_WAIT=14400 LC_SWEEP_WORKERS=1 bash scripts/dev/test-local.sh --browser-visual-lint` returned **1**, 294/294 images, three R6 instances. No source edit was made in response to this result.

## Exact evidence

- `output/ui-visual-audit/20261005T074421Z/lint.json` and `lint.md`.
- `shots/admin/products-product/en-1586x992.png` in that directory.
- `crops/admin/products-product/en-1586x992-R6-1.png` and `-R6-2.png` (two grouped records, three instances).
- Command log: `output/admin-visual/gates-20261005T074219Z/browser-visual-lint.log`.

## Finding

The axis value input and remove-axis control have raw rectangles beginning at y897.5 with height44, ending at y941.5. The field scrollport visibly clips them at y907. Footer buttons begin at y936; the footer surface begins around y924. Consequently the **raw** rectangles intersect by5.5px while their **visible** regions do not.

The frozen collector `tests/ui/visual-lint-lib.mjs:300–308` computes both raw and clipped rectangles. Interactive targets at line429 retain the raw `getBoundingClientRect()`. `r6Overlap` at lines185–193 compares those raw rectangles. Text collection, by contrast, already intersects its line rectangles with the clip at lines402–403.

Root and an independent read-only reviewer both inspected the exact screenshot and collector. The reviewer's arithmetic probe using the frozen `r6Overlap` function returned raw1 / visible0 (exit0, **MODEL_ONLY**). Actual live browser hit-testing for this discrepancy is **NOT_RUN**; the screenshot and source investigation are not presented as a replacement for the failed gate.

## Decision boundary

The current unit may not modify `tests/ui/**`. No arbitrary padding, sticky/fixed classification, hidden control, altered fixture, tolerance change, or discarded failure is acceptable. Ask the harness owner to adjudicate clipping semantics and provide an independently tested detector correction, or retain this gate as BLOCKED. Other functional gates continue on the unchanged source.

Independent memory: `0e6331ab-879a-4aae-a47a-ed080285f990`, title “R6 e856 independent raw-clip false-positive source and screenshot diagnosis”.

The earlier 5a895dea zero-R6 corpus is historical, not proof that every current settled geometry is clean. Preserve it and this red run.
