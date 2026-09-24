# Merchant account HTTP and BFF acceptance

2026-09-24. Code `8318069` (Go intake), `c2a3e4d` (new-owner grant fix),
`c59dd06` (BFF, independent PG/browser tests), `6054014` (actual API process restart),
`a5c4260` (failure-path cleanup and integrated tagged command).
The full untagged suite and separate tagged browser/process gates passed. Final
review also closed a failure-path test cleanup race; evidence is below.

## Scope and root causes

- Four account endpoints: create, safe paginated list, safe detail and CAS rotation.
  No public decrypt/export/delete/verify. Metadata never contains key ID or secrets.
- Existing store BFF now admits the account paths and five existing settings paths,
  including method PUT and read-only inspect POST. It reuses real HttpOnly sessions,
  authorized-store enumeration, Origin/CSRF and scoped backend authority.
- Configuration is default-off with strict encrypted-keyring input, separate stable
  replay key, literal-loopback Go listener and no worker start. See [runtime guide](account-credential-configuration.md).
- Initial signed-IdP browser run returned 403: migration 0007 bootstrapped pricing
  grants but 0008 only extended the permission vocabulary. **0019 adds read/manage
  only for newly created store owners**, no execute or existing-member backfill.
  A revoked grant is not resurrected by initial-store replay. Function owner, ACL,
  pinned search path and lock ordering remain tested.
- Browser Origin negative test initially returned 401 because Playwright's HTTP
  client did not automatically send Secure cookies on explicit HTTP loopback.
  The test now uses the exact server-issued isolated cookies, first proving a
  same-Origin 200 control and then changing only Origin to obtain 403. The assertion
  was not relaxed to accept the wrong denial cause.
- Go and BFF rate-limit errors now both mark 429 retryable, preserving bounded
  numeric Retry-After. Admission is tenant/store isolated, not a global 60/min quota.

## Gates

|Gate|Evidence and exact boundary|
|---|---|
|AC01|HTTP saves, safe list/detail, encrypted PG versions and rotation PASS. Final actual built `cmd/api` PID 91870 cleanly exited on SIGTERM; new PID 91898 loaded the same isolated PG/key environment, read/listed/replayed the saved account and rotated it. Both persisted versions independently decrypt correctly; no provider jobs/operations were created|
|AC02|Permanent replay/changed-secret conflict/CAS single winner, cross-store refusal and proven lock-wait permission revocation PASS; existing internal disabled-binding tests remain in full regression|
|AC03|Malformed JSON/queries/input, nil service, pagination binding, permission denial, limiter reset/capacity/concurrency and key configuration negatives PASS|
|AC04|Actual Chromium → packaged Next BFF → Go → isolated PG with signed MOCK IdP PASS: create/list/read/rotate/replay/CAS, 429 then successful read, CSRF/Origin/cross-store refusal; method PUT/replay/read, read-only inspect without command key and forbidden enable|
|AC05|302 top-level PASS / 0 FAIL / 0 SKIP in final full PG/race/vet script; TS and packaged build PASS. Separate final tagged gate: 2 PASS / 0 FAIL / 0 SKIP with race and standalone tagged vet exit 0. Independent review found no P0/P1; its P2 test cleanup race was fixed and re-reviewed|

Browser market records are explicitly seeded only in the isolated harness after
a successful real onboarding receipt. No permission or provider qualification is
seeded to make the account path pass. This is not market-discovery UI acceptance.
DB postflight proves one account, two immutable credential versions, two disabled
method revisions, zero qualifications/provider operations and no inspect receipt.

## Evidence

Root log directory: `/Volumes/data/output/live-commerce-account-intake-tests/`.

- `full-initial.log`: exit 0; 300 top-level PASS, foundation 167.787s; before 0019.
- `browser-initial.log`: exit 1; genuine new-owner permission gap retained.
- `browser-second.log`: exit 1; hostile-Origin test lacked its legitimate cookie.
- `full-final.log`: exit 0; 302 top-level PASS, foundation 179.999s; full race and
  silent successful vet under `set -e`, followed by final script PASS. Standalone
  `GOTOOLCHAIN=go1.27.1 go vet ./...` also returned 0.
- `browser-third.log`: exit 0; 1 browser chain PASS, foundation 6.835s;
  browser artifact `output/playwright/identity-chain-20260924T143437.709480000/`.
- `browser-process-final.log`: invocation exit 126 because this repository script
  is not executable; retained as evidence. The existing supported `bash` invocation
  was used for the following successful run, without changing file permissions.
- `browser-process-run.log`: actual exit 0; 2 top-level PASS / 0 FAIL / 0 SKIP,
  foundation 12.287s. Actual process restart 3.81s and real browser chain 6.47s.
  Artifacts: `output/playwright/account-process-20260924T144831.598399000/` and
  `output/playwright/identity-chain-20260924T144834.860166000/`. The API executable
  uses synthetic credentials, local signed MOCK IdP and restricted isolated PG;
  it never starts a worker. Both child PIDs and the DB fixture were removed by
  normal exact-target teardown. Build and process logs are retained.
- `browser-process-accepted.log`: actual exit 0 after independent review's P2
  cleanup race fix. Final 2 PASS / 0 FAIL / 0 SKIP; foundation 15.881s, process
  restart 4.80s and browser chain 8.06s. Artifacts:
  `output/playwright/account-process-20260924T145225.105624000/` and
  `output/playwright/identity-chain-20260924T145229.419429000/`.
  Cleanup now synchronizes only through the closed `done` channel rather than
  racing `cmd.Wait` to read `ProcessState`; timeout cleanup reports a test error.

```text
0db5a495c4c9678b57ff43255839452b2e09c22830eb9b6dbf1f72190b4f831f  full-initial.log
b70524fe66affc976c824d31be3db5feadc92c8cf79a22c391b4997408e50f0a  full-final.log
4233a8bdd911f433764894ee9bf8e504a6687d0ba668cf0296689096995fc4a0  browser-initial.log
2feaf5d5197307e15f873a217f07db7539c86f7c77da258af41145237f84ae81  browser-second.log
542cf735414797882dcbc5fcc77c85c78095d729ea60a90f9d95d5f0d89c5e1f  browser-third.log
fcfbeb9d751c482509cd9d01521f1fc795f063aebcb916bb99a8c44490ffb555  browser-process-run.log
ba4f4bd06fb1e597c24780d7ced5b39c1546315852ac02d9cee248ad683e03ea  browser-process-accepted.log
```

## Independent review and remaining work

Go author `account_intake_impl` (gpt-6-sol/high), isolated worktree, owned 12 files;
follow-up owned only 0019 and its PG test. Root owned BFF and independent integration
tests. Reviewer `account_intake_review` (gpt-6-sol/high) reported no additional
P0/P1 after the new-owner fix and explicitly caught the restart evidence gap.
Preflight `eb8bc06b-9484-4ab9-9c75-f44598315907`; implementation review
`bf2b1998-e948-4095-8d18-89244ebdd583`; restart boundary
`3330585b-32c6-4d1f-835f-e1221f6157f6`. Inventory/asset manifest used
`settings_api_inventory` (gpt-6-luna/medium). No recursive delegation.

Final independent accepted review: `89366cfd-a812-4a22-8d34-fc208d9a8728`,
"merchant account AC01 process restart review accepted; cleanup P2 closed".
It supersedes `1ea7d6c3-0ffa-480b-b0bf-d6e8b8219757`, whose non-blocking
failure-path `ProcessState` race was fixed in `a5c4260`. No open P0/P1/P2 was
reported in this bounded final review; this is not an audit of the entire SaaS.

User approved [A horizontal-step comp](../../.impeccable/mocks/merchant-settings-a.png);
[brief](../../.impeccable/merchant-settings-brief.md) pins that direction. This is
not yet the runtime wizard or its desktop/mobile/localization acceptance.
Real IdP, provider sandbox/live, key recovery/KMS, distributed abuse controls,
market/method discovery, provider qualification and full SaaS deployment remain
separate gates. Saved accounts remain `CONFIGURED_UNVERIFIED`, merchant payment
enable remains rejected. No client production settings or business was changed.
