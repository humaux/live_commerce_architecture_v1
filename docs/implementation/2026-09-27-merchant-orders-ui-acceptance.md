# Merchant orders C UI acceptance

Scope: local read-only `/{locale}/orders`, user-approved **C 表格原位展开**.
Buyer B is unchanged. This is not whole-SaaS, production, provider-payment,
shipping, refunds or live-broadcast acceptance.

## Implementation and authority

- Contract: [MOU01–06](../../contracts/merchant-orders-ui-v1.md); existing
  [MOR](../../contracts/merchant-orders-v1.md) and
  [MBT](../../contracts/merchant-orders-bff-v1.md) remain the server boundary.
- Source candidate `e4189ee`, root integration `1c27aec`: authenticated store
  selection, limit10 keyset pages, strict read DTOs, frozen financial/delivery
  details, independent commercial/payment/fulfillment/work states, three locales.
- Detail remains directly below the selected row. Mobile uses the same order
  and detail sequence with scoped item-table scrolling. No raster asset ships.
- Session/generation fencing and synchronous hide/pagehide clearing prevent
  old responses and cached recipient data from resurfacing. No persistent
  order-body cache, browser bearer or additional transaction engine.

## Evidence and current gate disposition

Current source/test tree: `f70dc42` (original source `e4189ee`, source fixes
through `6df6a03`, independent tests `13db0d2` integrated at `a891991`, native
history artifact persistence `4dad02a`, payment-column regression `f70dc42`).
**PARTIAL — full MOU acceptance remains blocked by native lifecycle evidence.**

Final root command `bash scripts/dev/test-local.sh --browser-merchant-orders-ui`
exited **1**, deliberately: 6 functional Playwright cases passed in 11.8s, one
native visibility case was skipped/NOT_RUN. Go foundation took 16.193s and
reported the incomplete native gate after checking unchanged PG business facts.
Log: `/Volumes/data/output/merchant-orders-c-visual-fix-gate-20260927.log`.
Evidence root: `/Volumes/data/output/merchant-orders-c-browser-20260927/20260926T195656.478109000/`.

| Gate | Observed disposition |
|---|---|
| MOU01 actual flow | PASS local signed-mock login → production Next → Go → PG; filter/pagination/selection/refresh |
| MOU02 authority/privacy | PASS local two principals/stores, invalid sessions, foreign/unlisted scope, storage/cache negatives |
| MOU03 lifecycle | PARTIAL: labeled delayed/error/pagehide injection, actual history and common-shell cross-tab logout pass; native hidden/visible NOT_RUN |
| MOU04 financial/history truth | PASS local state/amount/frozen-data checks and unchanged business facts; paid-allocation fault is explicitly controlled SQL, not a pure business-API fixture |
| MOU05 visual/accessibility | Three locales × desktop/mobile captured; target sizes, focus, no document overflow and payment-label geometry pass; independent correction verdict recorded below |
| MOU06 regression/record | Typecheck/build/parser and existing browser regressions pass at scopes below; no aggregate green claim while MOU03 is incomplete |

The native history artifact at `results/orders-ui-MOU03-controlled-ea3df-istory-and-cross-tab-logout/native-pageshow.json`
records `observed=true`, `persisted=false`, for `/en/settings` and `/en/orders`.
That proves an actual history return but **not native bfcache restoration**.
Headed tab-switch/minimize probes on this host still returned
`document.visibilityState="visible"`; even native window bounds reporting
minimized did not trigger the needed transition. Stop probing this host; replay
the existing native test on a capable browser environment. Do not replace it
with a synthetic event or set `LC_BROWSER_NATIVE_VISIBILITY=1` as an assertion
that the host supports it: that flag enables the real test, which must pass.

At `f70dc42`, `pnpm run typecheck:admin` and
`node --test --experimental-strip-types tests/admin/orders-model.test.ts` exited
0; parser tests 2/2 pass. The MOU runner also completed a production Next build.

After the shared logout repair, existing browser gates were replayed at the
`654272c`/`9f57668` source sequence (all exit 0; no later logout behavior edit):

| Command suffix | Result | Log under `/Volumes/data/output/` |
|---|---|---|
| `--browser-identity` | 3 PASS, foundation 13.750s | `merchant-orders-c-identity-final-20260927.log` |
| `--browser-merchant-buyer` | 1 PASS, 5.420s | `merchant-orders-c-buyer-final-20260927.log` |
| `--browser-merchant-orders-bff` | 1 PASS, 6.327s | `merchant-orders-c-bff-final-20260927.log` |

## Earlier retained evidence

Root `1c27aec`, all commands exited 0:

| Command | Observed result | Log under `/Volumes/data/output/` |
|---|---|---|
| `pnpm run typecheck:admin` | strict TS pass | terminal result |
| `bash scripts/dev/test-local.sh --browser-identity` | 3 top-level PASS, foundation 13.237s | `merchant-orders-c-identity-regression-20260927.log` |
| `bash scripts/dev/test-local.sh --browser-merchant-buyer` | 1 PASS, 6.521s | `merchant-orders-c-buyer-regression-20260927.log` |
| `bash scripts/dev/test-local.sh --browser-merchant-orders-bff` | 1 PASS, 7.758s | `merchant-orders-c-bff-regression-20260927.log` |

Each browser runner builds production Next and uses disposable PG18 plus a
signed MOCK OIDC issuer. It removes its own database fixture at exit.
The emitted production layout retains design-contract seed `549adef8`.

## Issues caught before acceptance

1. Strict DTO validation initially coerced values with `String(...)`; JSON
   arrays/numeric identifiers could pass regex checks. Source candidate requires
   actual strings before regex checks; independent parser negatives are required.
2. The first response fence checked generation before an awaited cookie digest
   but not after. The candidate rechecks generation/abort/visibility after await.
3. Real browser setup exposed fixture assumptions: payment quantities must obey
   the existing minor-unit contract; loopback `__Host-` cookies must be tested
   through browser fetch; two random store IDs cannot imply a fixed default.
4. Real merchant routes had no reachable Entry logout. A common-shell logout
   now uses the existing CSRF/session boundary and endpoint, emits clear-only
   cross-tab signals and respects `locked` during pending writes. Actual UI
   logout is exercised; synthetic API-plus-event is not its substitute.
5. Global product-table CSS leaked fixed column widths into nested order items
   and the mobile detail cell. Scoped column widths and mobile specificity fix
   both, with measured item bounds and mobile detail/recipient widths.
6. Independent finish review caught a separate fifth-column 9% inheritance that
   clipped desktop payment text. `f70dc42` sets that column to 20% and permits
   badge wrapping; three-locale desktop/mobile badge-bound assertions pass.

## Reviews and retained visual evidence

Bounded independent source review at `9f57668` found no remaining P0/P1/P2
within its code scope (Humaux `d39627b2-d659-418e-8838-3a1671e9df57`). That is not
a visual/native lifecycle approval. Independent test evidence is
`e477af54-a075-422c-8781-e7cc172f631e`.

The one detector invocation returned `[]` before the final CSS correction:
`/Volumes/data/output/merchant-orders-c-detector-20260927.json`.
It was not rerun or treated as visual acceptance.

Initial nine required screenshots and optional long captures are retained at
`.impeccable/review/merchant-orders-c-20260927/`; the final same-viewport captures
are in its `fix-round-1/` subdirectory. They use only synthetic local fixtures.
Independent initial verdict was **FIX**, solely the clipped payment state;
the same independent reviewer scored that P1 **RESOLVED** at `f70dc42` after
reading all nine final desktop/mobile/detail captures. This verdict covers that
fix list only, not a new whole-surface approval or MOU03. Humaux title:
`Merchant orders C fifth-column visual P1 resolved at f70dc42`.
Buyer B and the inherited visual world
are unchanged; no shipping raster assets were introduced.

## Cleanup and next acceptance action

The disposable PG runner's owned-container cleanup ran despite exit 1; a final
Docker name-filter readback returned no `lc-foundation-test-*` containers. No
customer service was stopped, no production data changed, no real payment sent.
Earlier failed runs and screenshots remain available for audit.

Next: complete native hide/visible restoration, retaining fresh-authorized-read
and no-PII assertions. The bfcache observation is refined below; do not remove
private/no-store responses to force a cache hit. Then replay MOU and update this
record. Do not infer whole-SaaS or deployment readiness.

## Native history harness diagnosis, 22:40 UTC

Installed Playwright 1.63.0 passes `--disable-back-forward-cache` by default
(`playwright-core/lib/coreBundle.js`, Chromium switches). This suite now omits
only that default, waits for history navigation **commit**, then polls the actual
pageshow record. A restored document need not emit another load event. No product
code, response header, native-visibility assertion or skip policy was changed.

A separate isolated loopback HTML probe emitted trusted native pageshow events
`/first:false → /second:false → /first:true` with that flag omitted and
`goBack({waitUntil:"commit"})`. Thus this host supports native bfcache for an
eligible document. This is a capability probe, **not** order-page acceptance.
Removing background-window/timer flags and disabling focus emulation on both
probe tabs did not produce a native hidden event; that hypothesis was rejected.

Actual order-chain rerun:

- Command: `bash scripts/dev/test-local.sh --browser-merchant-orders-ui`
- Log: `/Volumes/data/output/merchant-orders-native-bfcache-20260927-diagnostic.log`
  (SHA-256 `d1be8798aaaa63769e632cd050c88bcea03791c77859c2c750db5dd549a8bc4c`).
- Artifact root: `/Volumes/data/output/merchant-orders-c-browser-20260927/20260926T224016.147901000/`.
- Playwright: **6 passed, 1 skipped** (9.3 s). Go foundation **12.458 s**, aggregate
  **exit 1** because native visibility remains NOT_RUN. Not an aggregate pass.
- The actual order history return still records `observed=true,persisted=false`.
  Its browser-provided `notRestoredReasons` includes `masked`,
  `response-cache-control-no-store` and
  `response-cache-control-no-store-with-js-network-request`. These are reported
  reasons, not a claim that every browser will reject caching identically.
- `native-pageshow.json` SHA-256:
  `df9b2befc811d6ee27aca3ce31f87c7285ced968ee37ef2d2981eb984985fa98`.

The first changed harness run read the event before pageshow and failed its
existing defined-event assertion. That red log is retained as
`/Volumes/data/output/merchant-orders-native-bfcache-20260927-first.log`; bounded
polling fixed the event-order race, not the product. The second log is
`/Volumes/data/output/merchant-orders-native-bfcache-20260927-repair1.log`.
Native hidden/visible is still unresolved; do not retry the same host probes
without new evidence or relabel synthetic events as native acceptance.

Independent `live_draft_tests` read-only review accepted the bounded test-only
diff and checked both hashes above; no native-visibility or privacy assertion
was weakened. This does not close MOU03.

A separate empty-profile system Chrome channel probe (`153.0.8010.53`, headed,
focus emulation disabled on both tabs) also returned `visible` with no native
visibility events after both tab switches. Existing customer tabs/profiles were
not used. Merely choosing the installed Chrome binary does not close that gap.
