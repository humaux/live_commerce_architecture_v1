# PR31 per-thread replies

| Thread | Fix SHA | One-line reply |
| --- | --- | --- |
| 4231878479 | `3e5a9c4a` | Fixed in 3e5a9c4a: successful reminder receipts stay visible and report GETs require inbox:read; the regression asserts one POST and zero unauthorized GETs. |
| 4231878495 | `3e5a9c4a` + `c9608348` | Fixed in 3e5a9c4a and c9608348: same-address navigation preserves the privacy lifecycle, including an implicit default store; real scene/store/locale departures still clear synchronously. |
| 4231878505 | `3e5a9c4a` | Fixed in 3e5a9c4a: the picker consumes next_cursor and merges paged choices, while deep links load the selected scene's authoritative detail. |
| 4232189503 | `3e5a9c4a` | Covered by 3e5a9c4a: page-two scenes are selectable and deep links display the selected scene's authoritative title. |
| 4231878513 | `afe0317a` | Fixed in afe0317a: GET/POST preserve the 120-Unicode-rune name contract and legal lists beyond 500 rows under the existing 1 MiB bound; 121 runes remain rejected. |
| 4231878523 | `ac43103c` | Fixed in ac43103c: the sold-out sent badge requires confirmed delivery; rendered-component and browser matrices cover pending, failed, blocked and UNKNOWN states without a sent badge. |
| 4232031441 | `3e5a9c4a` + `c9608348`, `dea6b6aa`, `5b1da81a` | Fixed in 3e5a9c4a and hardened by c9608348/dea6b6aa/5b1da81a: proven unmounted reminder routes stay panel-local; actual permission or scope loss still clears private data. |
| 4232031446 | `3e5a9c4a` | Fixed in 3e5a9c4a: the empty placeholder is disabled after a scene is selected, including first-page and deep-linked scenes. |
| 4246128710 | `d05ab56e` | Fixed in d05ab56e: Studio not-found now uses the shared scoped-404 fence/clear path; a late private read cannot repaint and an old callback cannot POST, while a 503 retains authorized state. |

All eight prior fixes are current HEAD ancestors. No FOLLOWUPS proposed for these nine threads. The integrator posts replies and resolves; the author has done neither.
