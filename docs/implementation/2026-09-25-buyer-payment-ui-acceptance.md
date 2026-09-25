# B buyer payment UI — bounded acceptance

2026-09-25. **PASS_BOUNDED_LOCAL_BROWSER_MOCK** on `c149a20`; BPU04's two identified
visual fixes are independently resolved. This is **not** real-provider,
physical-device or deployment acceptance.
The approved B product-detail purchase composition is retained.

## Delivered boundary

- Current and historical owned orders share one payment section. A synchronous
  explicit click reserves an opener-isolated blank tab before asynchronous work.
- The existing purchase Web Lock serializes prepare, Take and competing purchase
  actions. Context, selected-order generation, immutable amount, pending journal,
  local marker identity and destination readiness are checked across awaits.
- Original prepare key/body can be explicitly replayed. The handoff-started marker
  is persisted before Take; any subsequent uncertainty permits reads only. Refresh,
  focus, reload and provider return never initiate another payment.
- Only the bounded validated form is submitted by a native POST. No form,
  credential, receiver information or payment-success claim is stored locally.
- A frozen test-mode flag remains visible. Payment/commercial states in the payment
  section come from the same authoritative projection; the older order summary is
  not merged into it. Submission is not capture, and capture is not bank settlement.
- The fixed `/payment/return` GET/POST handler ignores callback input and emits
  identical three-language neutral HTML. It has no tenant lookup, redirect,
  cookie, script, form or mutation; the buyer returns to the original store tab.

No new dependency, Go production source, SQL migration or merchant enablement.
Dependencies and re-gates are recorded in [dependencies.md](dependencies.md).
Contract: [BPU01–04](../../contracts/buyer-payment-ui-v1.md).

## Independent engineering and functional evidence

Controller author worked in a separate worktree (`73e1409`, `a751cf9`, integrated
as `6b81b17`, `af8feb8`). Root implemented UI, return handler and configuration.
Independent test author owned only Node/browser test files and the runner branch;
final author commits `aff19fa`, `5ce8795` became `542b252`, `a9354f1`.

| Gate | Root-observed result | Log under `/Volumes/data/output/` |
| --- | --- | --- |
| BPU01 + existing storefront Node | 55 PASS, 0 fail/skip; 9 independent controller cases plus neutral-return case | `buyer-payment-ui-root-node-6.log` |
| TypeScript strict check | exit 0 | `buyer-payment-ui-root-typecheck-5.log` |
| Production Next build | exit 0, rebuilt by run 7 before browser execution | `buyer-payment-ui-root-browser-7.log` |
| BPU02 actual Next → Go → isolated PG18 → native browser POST | 11 PASS, case 7.52s / package 9.793s, exit 0 | `buyer-payment-ui-root-browser-7.log` |
| Existing order/history browser regression | 23 PASS, 6 buyers / 7 orders, case 15.28s, exit 0 | `buyer-payment-ui-root-order-regression-2.log` |
| Backend PG18/race/vet regression | Reused 421 PASS / 0 fail/skip, 174.239s; production Go/SQL/modules byte-identical to accepted `ed90f9f` | `buyer-payment-public-root-full-1.log` |

The full backend suite was **not rerun** for this UI-only slice. The new Go test
harness and updated runner were actually executed by BPU02. The final order
regression includes the neutral-header fix, but precedes the currency/status-copy
fix `8a27351`. Run 7 rebuilt that fix and passed five new exact currency/status
assertions (author `66d6c69`, root `c149a20`). A handler-unit test alone did not
prove effective HTTP headers.

Browser cases cover fresh and historical orders, duplicate clicks and actual
cross-tab Web Lock queueing, blocked/closed/navigated child, post-Take child close,
committed prepare-response loss with exact replay, committed handoff-response loss
with GET-only reload/focus, actual buyer-context replacement before Take, effective
GET/POST return headers/body, three locales and touch-mobile Chromium.

The loss fixture obtains the backend's committed 200 response, then sends headers
and one byte with a longer Content-Length before destroying the response. It does
not merely abort before the write. The context test holds a committed prepare,
establishes a genuinely different buyer context, releases the response and proves
zero Take; only restoration plus explicit same-key/body replay proceeds.

Two actual native browser form POSTs are intercepted locally at the exact pinned
sandbox URL. The Go harness independently compares their canonical four-field
digests to persisted PG forms. There are four attempts/pages/issued results,
one query job per attempt, and zero financial facts or review records. There is
**no request to the real PSP**. Synthetic bearer cookies stay in process memory.

Mobile native history payment and neutral return use a separate Playwright Pixel 7
context with `isMobile`, touch and Android UA. Other 390px captures are responsive
desktop-width checks. This proves neither a physical phone nor Safari/WebKit.

Run-specific evidence:
`output/playwright/buyer-payment-229897813/` (`result.json`, browser/Next logs and
eight screenshots). Root visually inspected the full desktop order, narrow ready
and read-only panels, mobile native history and both neutral returns.

## Failures retained and corrected

1. Run 1: test fixture's 12.50 TWD order was ineligible for whole-TWD admission.
   Quantity became two; production monetary admission was not weakened.
2. Run 2: assertion raced actual child navigation after native POST. The harness
   now awaits the observed destination navigation before reading its state.
3. Run 3: pre-byte connection abort permitted a browser transparent POST retry.
   A committed partial response now causes the intended non-retryable loss.
4. Run 4: **actual application defect** — Next global headers overrode the neutral
   handler's Referrer-Policy/CSP. `91abbc9` excludes the neutral path from shopping
   headers while keeping shared safety headers. Actual GET and POST in runs 5/6
   prove no-referrer, no-store, restrictive CSP, no redirect and no cookie.
5. Run 5: ten gates passed; responsive-width checks were not called mobile-device
   acceptance. Run 6 added the actual touch context and buyer replacement case.

6. Run 7: post-visual-fix 11 gates passed; explicit TWD and separate payment/order
   status labels are independently asserted on the rendered desktop/mobile UI.

Logs `buyer-payment-ui-root-browser-{1..7}.log` and all failed run directories are
retained, not rewritten. Prior tracked baseline order screenshots were restored
only after their newly captured equivalents were verified in the run directory.

## Independent review and visual boundary

- Source review `8381f1c1-7852-4bce-abcf-b3265eaac95a` through `af8feb8`: no unresolved
  financial P0/P1; truthful submission copy corrected the identified P2.
- Final test-causality review `6db9d069-f344-41bb-9ffd-121192df5b9d` at `a9354f1`:
  no obvious new false-pass path; source review and log inspection, not a second
  independent execution of the complete browser suite.
- Test-author evidence `1dfaec67-44eb-400a-a959-d5680b5aefd8`; root reran all gates.
  Final visual-label test evidence: `bd3dc950-d6b6-4c2e-bc84-5ef79711c220`.
- One targeted Impeccable detector pass found only the inherited Arial font
  advisory; no global typography redesign is authorized or warranted.
- Fresh review `85ae8e24-53a1-40c8-ab2f-65d5034e197d` identified two fixes: an
  ambiguous local `$` for zh-TW TWD, and adjacent unlabeled duplicate states.
  `8a27351` makes only zh-TW TWD use its explicit currency code, and labels both
  states without merging different projections. Verdict
  `c162f2fe-f623-478f-b81d-fe5469e9c7d8` scores **both fixes resolved** and `ship`
  at that scope, with no visible fix-batch regression across eight run 7 captures.
  This is not a new whole-surface audit. No second detector or unrelated redesign.
  Local [built design record](2026-09-25-buyer-payment-ui-design-record.md) records
  the extension rather than replacing the incumbent system.
  Global `DESIGN.md` and `.impeccable/design.json` are unchanged.

## Evidence checksums

| Artifact | SHA-256 |
| --- | --- |
| Browser run 7 | `0f90ed3c6f963dac162ef28ff0c45f452252c9917cb60d2c6f357a635033cd5e` |
| Node run 6 | `0ec526b4d8c937a5cb0fce4eb09e09de71887ae8609acba190b3b08c269c8f7e` |
| Typecheck run 5 | `de46600b5fb7fd9cd4206a8374bb2562031f87e6f5f259353f2740f3def3cd6f` |
| Existing order regression | `cd3275e413d698032057386e55e890efa4203f769a2eb2dfc24915ae135d3d54` |
| Detector JSON | `5b032771b21561bab6b7a4744b50b46e40f44c8c27125c6617adc70b71145eee` |
| Run 7 result.json | `5d8192634e9c69c0bbc249563747219717d0fac231f155eba07431db9601620f` |

Owned browser, private HTTP and ephemeral PG fixtures exited after the gates.
No task-labelled Docker fixture remains. Pre-existing unrelated Next servers
were left running. Capture evidence remains in its run-specific directories.

## Still closed / not established

Real provider qualification, production query/capture worker and notification
assembly, the configured absolute central ReturnURL's DNS/TLS routing, customer
credentials and sandbox/live acceptance, refunds/reconciliation and full SaaS
release gates remain open. No customer production service or live broadcast was
changed. Merchant payment enablement remains closed. An uncertain handoff must
not automatically create another attempt or release held stock.

Next-unit preflight `38ee7322-6080-43c4-9a29-6c9e32d3ddd9` found that payment,
expiry and external-operation jobs currently share River's default queue. Do not
start a worker registering only payment kinds on that queue. Freeze dedicated
payment routing and treatment of existing jobs before assembly; keep notification
ACK and real provider protocol qualification as separate gates.
