## verdict

1. resolved — Fix 4 now has rendered state evidence: `selection-active.png` visibly shows the mint selection background and dark ledger text; `caret-active.png` visibly shows the teal caret after the selection collapses; `active-style-evidence.json` records the real focused selection range, computed colors, and a naturally overflowing 740px window with `scrollWidth` 617 > `clientWidth` 549, `scrollLeft` 68, the authored scrollbar color, and `thin` width. `scrollbar-active.png` is the post-scroll region; the macOS overlay scrollbar's automatic hiding does not invalidate the measured active scroll state. The dedicated browser-state test passed 1/1 in 1.5s.
2. resolved — the Next `N` regression is absent from the refreshed 1586×992 desktop and hero captures and the 390px full-page mobile capture. `devIndicators: false` is persisted in the installed Next 16.3.5 configuration, and no merchant control or product row is obscured.

Regressions introduced by this fix batch: none visible in the scoped recaptures.

## remaining

clear — both scored remaining items are resolved; this `ship` disposition covers the scored fix list and does not reopen the whole surface review.

disposition: ship
