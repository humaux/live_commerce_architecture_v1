# PR365 SU07 fourth-newOrder diagnostic candidate

- task_id: `bcd00080-ec44-4dc0-830f-df59fa006965`; child canvas `bcd00080-sub-1`. Parent claimed; no separate claim or delegation.
- role: tests-only diagnostic author. Requested `gpt-6.1-sol`, reasoning `high`; actual provider model ID UNKNOWN.
- base_commit: `5c4f3a504c546678b737a8d241b5d950791e1d4d`.
- branch: `codex/pr365-webkit-diag-1034`; worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr365-webkit-diag-1034`.
- only change paths: `tests/storefront/stripe-browser.mjs`, this report. Local commit only; parent owns integration and independent acceptance.
- evidence level: **E1 structural candidate**. Runtime capture, causal RED/GREEN, product fix, independent approval, SANDBOX and LIVE: **NOT_RUN**. Original failed SU07 remains a blocker; a future PASS with added observation would not prove the original race fixed.

## Authority and failure boundary

Parent card authorizes only first-observation tests-side diagnostics. Read AGENTS.md, architecture §§0,1,2,26,27, tasks T11 DAG/dependencies/gates, invariants (especially I02/I05/I06/I11/I14/I18/I21/I23), frozen stripe-buyer-ui SU07 and current purchase/checkout direct callers. Development rule `7bdb220b-0798-4b1d-aff5-16ee94cae75f` read in full. Rejected representative lookup covered 3/9, coverage .3333, not exhaustive. No matching existing bounded SU07 recorder was found in scoped tests search; existing `within`, evidence directory, privacy `hits`/scan are reused. Symbol tools unavailable; scoped rg/source reads were the equivalent, no index mutation.

SOURCE report: `/Volumes/data/live_commerce_architecture_v1/output/integrator/tools/logs/ci-evidence-1238/365-webkit-source-1023.md`, SHA256 `1bf9be508c122eecd7b5ddb82bfea671555229c6f6c001800212a4682ef2e199` (verified). Original execution `561a21bbb8ac4e500297dfcde779bf4230ffee72`, G41424128589, job125208467075, artifact13131669554. Existing checkout-empty assertion failed at the fourth newOrder after three SU07 passes. Click completion is established by reaching the subsequent assertion; React handler entry, accepted guard, lock acquisition and cart-version relationship remain UNKNOWN. No hydration/cart-version diagnosis is asserted.

## Exact added behavior

The single diagnostic starts after the existing worker-stop immediately before SU07 step 4 newOrder. It ends in the old continuation prefix finally, before reachCheckout and subsequent new order setup. All other newOrder invocations have no diagnostic evaluate or wait. Existing goto, 20,000 ms restore expectation, real locator click, empty-cart expectation, fixtures, money checks, keys/bodies, one-session/one-prepare assertions, retries and original error are retained. The former one-line conditional is reflowed to insert reads; its click/count/expect expressions are byte-identical. No product files or original oracle are changed.

Phases increment locally: existing-restore-UI-ready, click-before, click-after, empty-or-error (when the prefix completes), failure (when an original operation throws), finish. The original caught error is rethrown, then the preexisting outer failure handler receives it. Diagnostic evaluate/cleanup/write exceptions are swallowed to fixed capture/write flags/messages; no raw error is emitted and none replaces the original error. `within(...,1000,null)` bounds each read using the existing helper; this is a measurement bound, not a changed action/assertion timeout. Maximum normal reads are five; failure paths at most six. Observation adds scheduling/measurement overhead and can perturb a race. It is not a repair.

- One observed page has local pageOrdinal=1. Browser-local recorder identity assigns documentOrdinal on first sample; the same document retains it. Main-frame navigationOrdinal is separate and does not prove a new document. Before the first successful document sample, documentOrdinal=0 means UNKNOWN. Requests retain their initiation phase, requestOrdinal, initiation document (0 if unknown) and navigation ordinal; completion also carries the current observer phase. Unobserved older responses use requestOrdinal/documentOrdinal 0 and `untracked-response`, never a guessed new-document identity. No cross-stream total time ordering is claimed.
- Node observer stream: maximum 80 events and 160 request associations. Only same-origin main-frame requests/responses are observed. Methods are a fixed enum; route outputs are literal session/cart/order/payment/action/checkout templates or `other`. Status is Playwright numeric status; request failure outputs only aborted/timeout/network-or-other. No URL/query, body, header, cookie, key, context or identifier is serialized.
- Page click stream: maximum 24 native capture/bubble events, with local sequence/phase/document ordinal and disabled/order-presence/isTrusted booleans. Passive listeners do not dispatch, prevent, retry or force an action. Bounded data is copied into a closed scalar schema on the Node side; no arbitrary page object is serialized. Capture/bubble does not establish React handler acceptance.
- Maximum 8 snapshots: button/disabled/order/empty/error presence, aggregate checkout aria-busy, payment aria-busy, document-complete, storage-readable, locator presence/equality and journal presence/fixed kind. These are presence/DOM flags, not visibility or React readiness assertions. Checkout busy is aggregate, not isolated loading/working. Storage reads are at most 64 entries, locator JSON <=1 KiB and matched pending JSON <=64 KiB. Locator equality is only matching persisted candidate to displayed order, not schema/context validation. Storage errors/limits mark storageReadable false. No value/ID/key/body is returned. Cart-version same/newer and handlerAccepted/lockAcquired are explicitly UNKNOWN because this card does not inspect response bodies or product internals.

Runtime output is `LC_STRIPE_EVIDENCE/su07-continuation-diagnostic.json` (0600). The existing privacy sentinel `hits` is checked before writing on both success and failure paths; the file is also added to existing scanSurfaces/finalScan. A write/privacy rejection yields a fixed log message and missing diagnostic, never PASS. On ordinary completion Node listeners and WeakMap ownership are removed; page click listeners/global are removed by finish. A wedged/destroyed execution context may prevent page cleanup (captureFailed); context closure already performed by the original driver is the final cleanup boundary. No new full trace, screenshot, raw console or DOM export is added. Watchdog-before-finally may leave the diagnostic uncaptured; that is NOT_CAPTURED, not evidence of no request.

## Actual commands and receipts

All commands ran only in the isolated worktree unless the SOURCE report absolute path is shown. No Go, PG, provider, build, browser, CI or server process was started. No ownership generation, push, CI rerun or merge occurred.

| Command / action | Actual exit |
|---|---:|
| git status --short; git rev-parse HEAD; scoped rg/sed/cat source/contract reads | 0 except the read misses below |
| source report `shasum -a 256` | 0, matches hash above |
| node --check tests/storefront/stripe-browser.mjs | 0 |
| git diff --check | 0 |
| Python original-source byte reconstruction (below) | 0 |
| Python static diagnostic boundaries (below) | 0 |
| shasum -a 256 tests/storefront/stripe-browser.mjs | 0 |
| initial Python tasks read assuming dict | 1, list shape corrected read-only; no mutation |
| sed SOURCE path relative to isolated tree | 1, absent there; corrected absolute root report read/hash 0 |

Source SHA256 after static checks: `37d816c1975aa078b03432eb835aac6ef22688196c2db32f6687d5de903c5ef4`. Original baseline source SHA256: `eb067cbf66d65a95f73418bcd1f2d412e50271a1eeddd3973f02f4dfc2359512`. Final report hash and local commit are delivered outside this self-referential file.

The following source-only checks are reproducible; they never import/run the driver:

```sh
node --check tests/storefront/stripe-browser.mjs
git diff --check
python3 - <<'PY_CHECK'
import subprocess,re,hashlib
p='tests/storefront/stripe-browser.mjs'
base=subprocess.check_output(['git','show','5c4f3a504c546678b737a8d241b5d950791e1d4d:'+p],text=True)
s=open(p).read();h=s[s.index('// SU07 fourth newOrder only:'):s.index('// ---- store-tab helpers ----')]
for forbidden in ['fetch(','.route(','.json(','.body(','.headers(','.cookies(','.setItem(','.removeItem(','dispatchEvent','preventDefault','navigator.locks','.screenshot(','.content(','.innerText(']:
 assert forbidden not in h,forbidden
for required in ['out.events.length>=80','out.snapshots.length<8','d.events.length>=24','requestOrdinal>=160','if(localStorage.length>64)','raw.length>1024','pending.length<=65536','JSON.stringify(out)','if(hits(raw).length)']:
 assert required in h,required
assert s.count('  startSU07Diagnostic(page);')==1
restored=s.replace(h,'')
restored=restored.replace('  const diagnostic=su07DiagnosticPages.get(page);\n  try{\n','')
restored=re.sub(r'^  if\(diagnostic\)await diagnostic.sample\("(?:existing-restore-UI-ready|empty-or-error)"\);\n','',restored,flags=re.M)
a='''  if(await page.getByTestId("continue-shopping").count()){
    if(diagnostic)await diagnostic.sample("click-before");
    await page.getByTestId("continue-shopping").click();
    if(diagnostic)await diagnostic.sample("click-after");
    await ex(page.getByTestId("checkout-empty")).toBeVisible();
  } // let the continuation settle before navigating away'''
b='''  if(await page.getByTestId("continue-shopping").count()){await page.getByTestId("continue-shopping").click();await ex(page.getByTestId("checkout-empty")).toBeVisible();} // let the continuation settle before navigating away'''
assert a in restored;restored=restored.replace(a,b)
restored=restored.replace('  }catch(error){if(diagnostic)await diagnostic.sample("failure");throw error;}\n  finally{if(diagnostic)await diagnostic.finish();}\n','')
restored=restored.replace('  startSU07Diagnostic(page);\n','')
restored=restored.replace('  if(cfg.kind==="SU07")s["su07-continuation-diagnostic.json"]=await readFile(path.join(evidence,"su07-continuation-diagnostic.json"),"utf8").catch(()=>"");\n','')
assert restored==base,'non-diagnostic source changed'
for expr in ['await page.goto(checkout);','await ex(page.getByTestId("continue-shopping").or(page.getByTestId("checkout-empty")).first()).toBeVisible({timeout:20000});','await page.getByTestId("continue-shopping").click();','await ex(page.getByTestId("checkout-empty")).toBeVisible();']:
 assert expr in s
print('ORIGINAL_SOURCE_BYTE_RECONSTRUCTION PASS',hashlib.sha256(base.encode()).hexdigest())
print('ORIGINAL_NAV_WAIT_CLICK_EMPTY_EXPRESSIONS BYTE_IDENTICAL')
print('E1 STATIC_DIAGNOSTIC_BOUNDARIES PASS; not runtime or privacy certification')
PY_CHECK
```

## Remaining scope and handoff

No runtime diagnostic JSON exists yet. The candidate only enables a separately authorized pinned run; root must inspect the diff/whitelist, independently rerun static checks, integrate as appropriate and register any next run with deadline/log/wakeup. No automated RED/GREEN is claimed for this diagnostic; no production issue is fixed. A stopped request chain plus accepted native click still cannot distinguish React early return, handler omission, lock wait or internal journal/context rejection. Product hooks require a new explicit card if later evidence needs them. P1 original failed gate remains open; no demonstrated new P0. All original failing evidence is preserved. WAIT_RUN NONE, no task-owned process/container/port/fixture was created. Only the two authorized files changed; no temporary script left behind.
