<!-- Purpose: PR #16 round-one corrective batch, current-source evidence and inherited-output preservation.
Depends on: comments 4221142374/4221142386, the real React/transport tests, local browser runners and SOURCE-SHA256.txt.
Used by: integrator pre-push review and PR #16; no production or provider acceptance claim. -->
# PR #16 round 1 delivery

- Branch: `unit/pr1-ui-followups`; tested source **`528b2b1c076fb86c319cd7af9708417058bbcbc3`**. Final delivery commit is evidence-only.
- Took over existing author head `56be1005`; fetched/merged own origin branch and trunk `4574fea9` (merge `8574e074`). No force-push or remote writes.
- Author: Codex-1 (exact host model identifier unavailable); no new delegates. Allowed scope: the two review issues, their tests, and necessary root UI fixes exposed by those tests.

## Both findings closed locally

**4221142374 — real readiness interactions.** `tests/admin/product-editor.acceptance.ts` now clicks the toggle at 390px in zh-TW/zh-CN/en and asserts `aria-expanded` false → true → false, list visibility/hidden state, and every missing entry's actual focused field inside its controlled section. The existing axe serious/critical assertion is unchanged. The final click ledger contains **30 readiness entries**: 27 missing-item interactions plus three locale toggle sequences.

The new test exposed real usability defects, not just missing coverage: the expanded index consumed the fields row, and English media hints could place the first control below the shorter pane. `ProductDocument.css` bounds the mobile index with its own scrolling; `useProductEditorLayout.ts` reveals the focused control inside the field pane without scrolling the outer shell. Desktop geometry and draft/write semantics are unchanged. Logical commits: `f8b85ba6` (coverage/cart), `fc226c7c` (bounded index), `528b2b1c` (field reveal).

**4221142386 — rotated buyer context.** `CartProvider` uses one `adoptContext` boundary in refresh and mutation. A changed context clears the previous cart/count and order locator **before** publishing the new context; reads also avoid applying to a superseded local context. The obtained context remains available before writes, so lost acknowledgements still retry B's original journal/context/Idempotency-Key/body. Tests run the actual provider with the real buyer-client/purchase journal and only the HTTP edge faked. They cover B's failed read, B's lost write/identical retry, focus-style refresh after rotation, and the pre-existing first-write retry.

## Red → green

- `node --test --experimental-strip-types apps/storefront/tests/cart-provider-retry.test.mjs`: **exit 1**, three rotated-context failures / one prior test pass (`cart-red.log`) → **exit 0, 4/4** (`cart-green.log`).
- Readiness mutation: temporarily disabled only the existing toggle's `onClick`; real `--browser-product-editor` failed at the new `aria-expanded=true` assertion after the click (`readiness-toggle-red.log`, `evidence/readiness-mutation-playwright.log`). The original ProductDocumentForm was restored and SHA-verified (`readiness-original.sha256`); **the mutant was never committed**. Other eleven tests in that serial sub-suite did not run, not PASS; separate CC12 still passed.
- Restored code then failed the new actual-field visibility assertion (`product-editor.log`). Bounding the index fixed Chinese but exposed English hint-height clipping (`product-editor-bounded.log`). Pane-local control reveal made the unchanged assertion green (`product-editor-final.log`). No retries/timeouts/thresholds were raised and no assertion was weakened.

## Final gates on 528b2b1c

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` | 0 | `node-final.log`: **1154 tests** |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc-final.log` |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | `storefront-tsc-final.log` (additional changed-app check) |
| `bash scripts/dev/check-gates.sh` | 0 | `gates-final.log`: 82 modes, header/architecture checks |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-product-editor` | 0 | `product-editor-final.log`: **12/12** product tests + **2/2** frozen CC12; real Next/Go/PG, MOCK identity |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-order` | 0 | `browser-order.log`: **26 cases**, exact order/hold/job/receipt/reserve counts; real Next/Go/PG |

Browser modes ran strictly serially. Runtime source stayed fixed during each run; `hash-verification.log` checks the changed source against `SOURCE-SHA256.txt`. All owned gate commands and harness services ended.

Final original browser evidence:
- Product: `output/playwright/catalog-core/20261008T170028.629379000/`; CC12 `output/playwright/catalog-core/20261008T170104.082957000/`.
- Order: `output/playwright/buyer-order-194863787/`.
- Committed subset: `evidence/product-click-ledger.json`, Playwright logs and the two root-defect screenshots. No actual buyer/production data or credentials were used.

## Preservation and remaining gates

At takeover this worktree already contained modified/untracked evidence in `output/product-ui-v2`, `output/product-ui-v2-fix` and `output/k28-pr1-ui-followups`. These were archived before testing, restored byte-for-byte after copying this run's evidence, and directory-compared with **exit 0** (`preservation-check.log`). They are intentionally **not included** in this corrective commit. The original backup `preexisting-output.tar.gz` and status snapshots remain local/untracked for recovery; inherited dirty evidence is not a claim that source is dirty. No other worktree or shared cache was cleaned.

NOT_RUN: independent review/required PR CI on this new head, full G07/foundation (PROCESS requires CI/integration coverage for storefront runtime changes; not run on the Mac), LIVE/provider/payment/deployment acceptance. The Node runner explicitly reports the unrelated R04 media binary gate NOT_RUN because `COMMERCE_R04_LIVEKIT_BINARY` is unset. Evidence level **E3 for the requested local batch**, not merge/production approval. Integrator owns push/review/merge; author stops here.
