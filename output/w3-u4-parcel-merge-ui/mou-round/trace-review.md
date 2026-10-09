# PR2 fbdf8ebb MOU03 trace review

task_id: 0cb19bd4-sub-mou-fbdf-trace; parent0cb19bd4. base_commit: fbdf8ebb0d1e091230fae7a646ce5405b84b2c50. Read-only evidence task; no code or test changes. Own output: this report only.

## Confirmed first blocking operation

orders-ui.spec.ts:493 final otherTab.close(), trace1-trace.trace call@633 starts16763.253ms and has no after/end record. Test timeout30000ms fires; teardown starts43682.293ms (~26.919sec after close). All preceding MOU03 expectations pass: controlled older detail invalidation, pagehide clear, history native-pageshow attachment, merchant-orders visible16234.200, restored detail Synthetic Buyer16571.519, Accountclick16658.979, signoutclick16729.277, primary order-detail count0 16745.875, body noPII16751.615, final detail count0 16761.351. Thus test did not first block at oldSeen/history/logout/privacy or parcel teardown.

## Actual network/lifecycle timing

1-trace.network: POST/api/auth/logout starts16724.215 status204 duration15.861; replacement GET/en/ starts16737.832 status308 duration4.460; GET/en starts16743.249 status200 duration16.775 (HAR body completion16760.024). close starts16763.253, about3.229ms after that recorded body completion. Fourteen replacement CSS/JS records start16771.436..16776.412 AFTER close, all200 ~1.0ms. HAR durations/starts are network recorder timestamps; these are not exact JavaScript callback or document-commit timestamps.

FullHAR241rows has no request duration>1000ms. No pageError captured. Trace has no explicit framenavigated/load/pageclose completion event proving replacement readiness beforeclose; last otherTab frame snapshot16731.647 reflects pre-replacement/en, last primary snapshot16762.336 reflects expired session. A200document response/body receipt must not be treated as committed/loaded document. Pending request start not captured by HAR cannot be excluded; no evidence of an HTTP request taking the remaining27seconds.

native-pageshow.json observed=true persisted=false, events settingsfalse then ordersfalse; notRestoredReasons cache-control-no-store plus masked. Native history was observed but bfcache restoration was not claimed.

## Source seam and limits

WorkspaceFrame.tsx:178-185 awaits logoutWorkspace then window.location.replace(/en/). This naturally creates replacement navigation independent of primary tab privacy-cleared assertions. Trace demonstrates Page.close overlaps replacement lifecycle, not a proven server/product defect. Customers-client lineage, parcel changes, renderer close mechanism and exact commit/load order remain UNKNOWN from this artifact. Do not modify MOU03 waits/assertions or infer a product patch solely from lineage. Parent unchanged-baseline observation can discriminate reproducibility; this reviewer starts no runtime.

Evidence root: /private/tmp/claude-501/-Volumes-data-live-commerce-architecture-v1/d13e4b5b-6b4d-4249-8530-ca9ae1ef506c/scratchpad/pr2-mou/playwright/merchant-orders-c-browser/20261008T082523.051016483/results/orders-ui-MOU03-controlled-ea3df-istory-and-cross-tab-logout/trace.zip
SHA256:e216be0ad262f83872ebc6a931cbaa7718f51d2b50360a0fba6dec89416a29f0.

Original trace/network/native attachment/log/source readable; Humaux historical entries used as index only; real accounts NOT_RUN. Actual commands rg/sed/gitrev-parse and Pythonzipfile/json/hashlib extraction exit0. NoGo/PG/browser/build/tests/production action. Existing runtime artifact reports1failed9passed, not a green acceptance. Browser-internal cause UNKNOWN.

Prior accepted WebKit review wording corrected from union/rebase to merge/union, per owner instruction; no source changes.
