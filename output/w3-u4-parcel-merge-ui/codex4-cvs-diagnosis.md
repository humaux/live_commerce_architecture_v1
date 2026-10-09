# PR2 CVS evidence: root cause not yet established

Source `b4495ba2`, CI run `37654972345`. The inspected trace's real long step is `taiwan-cvs.spec.ts:159` (`await tab.close()`): 9086.042–181509.396 ms. The following goto at line54 takes only 181511.351–181518.409 ms after the 180-second test deadline. Reporting this as a proven orders-page load loop would be inaccurate.

The print page, actual print-form and MOCK ECPay POST completed 200 before close. The print page uses a real top-level POST form; no PDF, window.print, document stream or beforeunload path was found. Trace has 12 orders reads and 9 of each parcel read across roughly three minutes; the product poll interval is 20 seconds. This does not show an unbounded parcel render/request loop. Popup frame snapshots/CDP close internals are absent, so product versus browser-infrastructure attribution remains UNKNOWN.

Original, unmodified b449 source passed local `bash scripts/dev/test-local.sh --browser-cvs`, exit0 in101.910s: Chromium MOCK and WebKit passed; SANDBOX skipped. Exact command and evidence: `codex4-cvs-baseline.json/log`. Raw baseline artifacts remain in the assigned worktree and are copied into main checkout before handoff. Original CI trace is retained in ignored `output/playwright/pr2-ci-37654972345/trace.zip`, with its SHA256 in the companion JSON.

No existing wait condition or timeout has been changed; no unsupported product print workaround is proposed. The confirmed permission teardown defect is fixed separately. The integrator has been asked whether to retain this issue as UNKNOWN for same-SHA Linux CI verification. No production, real carrier or live-key acceptance is claimed.
