# Independent final shared visual review — e85654d0

Reviewer: av-foundation, independent read-only shared presentation reviewer. Source worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/admin-visual`. Parent-pinned source: `e85654d00f684a855f7600d5af94a3ac29c67f46`. No recursive delegation, source edits, test/build/browser/PG/detector runs, or root output writes.

## Scoped verdict

No confirmed P1/P2 visual regression in the 48 directly opened current screenshots listed below. This is not acceptance of all 35 ADM findings, all 294 screenshots, or the full release. The frozen editor R6 discrepancy is assigned to another reviewer and is not adjudicated here.

- ADM29: six inventory screenshots show readable product names and single SKU treatment. Desktop retains stock/status/Select. Mobile has a localized horizontal-scroll cue and no duplicate SKU. Source (`globals.css`, mobile ledger override from 96286773) restores all table columns with `display:table-cell` and hides the duplicate mobile subline. The initial mobile screenshot shows only the leftmost product/SKU columns; actual dragging to Available/Select is NOT_RUN by this reviewer, not proven merely by the still.
- ADM21/R5 five genuine target shapes: reviewed mobile dashboard order links, dashboard All orders, customer-order links, Ads CAPI checkbox label and settings domain-origin link across all three locales. Enlarged link rows/spacing do not introduce clipping, overlap or unrelated layout drift. Source uses `--ui-target` for those exact surfaces. Exact DOM hitbox measurement and real touch/keyboard use are NOT_RUN by this reviewer; parent-reported R5=0 is not independently rerun.
- ADM31: six attribution screenshots now show a dash in the empty disabled draft selector and one localized no-draft explanation beneath it. The former duplicated option/hint text is gone. Distinct timeline/buyer/source information remains understandable. Ads budget/report caveat repetition is preserved as a frozen contract exception per parent, not silently called deduplicated.
- ADM35: all 24 actual CVS captures (two browser groups × three locales × two widths × two states) show real admin order/print-opening DOM. None is the old blocked-request placeholder or oversized 980-wide blank document. Merchant-created captures show an expanded carrier-created order with print action; opening captures show localized pre-response status. They do not show or accept provider label HTML, paper layout, printer output or manual-ready variants.

## Limited notes, not new blocking mandates

- WebKit CVS opening screenshots visibly use a compact native locale select face inside the larger topbar, unlike Chromium. A still cannot establish its actual clickable box; do not certify this select's 44px hitbox from these images. No overlap or unusable label was observed.
- CVS H1 says “Print store label” while the breadcrumb says “Print shipping label” (Chinese similarly distinguishes store label/shipping label). This is outside the original ADM06 named-route mismatch samples; recorded as wording variation, not a new P1/P2.
- ADM22 focused native date/calendar remains OS/browser-controlled NOT_RUN. ADM27 retains collapsed secondary filters and an export privacy explanation intentionally; no demand to remove the required explanation or redesign routing.

## Exact current screenshot inventory

Frozen visual corpus root:
`/Volumes/data/live_commerce_architecture_v1/.worktrees/admin-visual/output/ui-visual-audit/20261005T074421Z/shots/admin`

Directly opened 24 files using these exact Cartesian sets (each unique combination was opened):

| Routes | Locales | Sizes | Files |
| --- | --- | --- | --- |
| `inventory`, `ads-attribution` | `zh-TW`, `zh-CN`, `en` | `1586x992`, `390x844` | `<route>/<locale>-<size>.png` (12) |
| `home`, `customers-customer`, `ads`, `settings` | `zh-TW`, `zh-CN`, `en` | `390x844` | `<route>/<locale>-390x844.png` (12) |

Actual CVS capture directories:

- `/Volumes/data/live_commerce_architecture_v1/.worktrees/admin-visual/output/admin-visual/cvs-print/2026-10-05T07-43-12-872Z`
- `/Volumes/data/live_commerce_architecture_v1/.worktrees/admin-visual/output/admin-visual/cvs-print/2026-10-05T07-43-55-025Z`

Directly opened every combination in each directory: `opening-pre-response` and `merchant-created` × `zh-TW`, `zh-CN`, `en` × `1586x992`, `390x844`; filenames `<state>-<locale>-<size>.png` (24). Read first group's manifest scope: actual admin DOM; provider response existing MOCK; ProviderLayout/ManualReadyLayout NOT_RUN, SignedFields NOT_RECORDED. Browser-group names are parent provenance, not an independent browser invocation.

## Source/evidence examined

Read-only targeted source: mobile-target commit 96286773 globals.css patch; current Attribution.tsx248–277; prior independent 1ded gap audit and original REVIEW-admin.md ADM35 section570–582. Current own review did not reopen every source file or every old screenshot. Historical 063819Z 64-shot review is a separate source pin and is not counted in these 48 current files.

Skill: web-design-guidelines, freshly read locally and guidelines fetched from `https://raw.githubusercontent.com/vercel-labs/web-interface-guidelines/main/command.md` at 2026-10-05 during 07:59 UTC; retrieval returned 169 lines, revision/digest unknown. Relevant criteria: visible focus, labels, meaningful empty states, non-obscured controls, safe overflow. Raw source body is not retained here; latest full-guideline compliance is NOT_RUN and not claimed. Project frozen gates/brief take precedence over generic redesign suggestions.

## NOT_RUN / release boundary

All tests, builds, browser interaction, PG, detector execution; mobile table dragging and keyboard reachability; exact hitbox measurements; native focused date/calendar; provider/manual-ready print layout and physical printing; all remaining 35 ADM contexts and recovery/role states. Frozen lint result is parent-reported: 294 captures, admin R2/R3/R4/R5 zero, editor R6 three unresolved pending independent adjudication. This reviewer does not waive that gate.
