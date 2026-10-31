# RVFIX2-403 relay 2

CI head ad20a4af, run 41229383264: failed bundle:06 (--browser-identity), receipts, required. Foundation passed (no ownership drift).

| failing job/test | class | change |
|---|---|---|
| bundle:06 `tests/admin/settings-real.spec.ts:29` via settings-v2.spec.ts:638 `expect(save).toBeEnabled()` after the 403: Save pricing policy stayed disabled | b (my fix ad20a4af broke the denied-retry path) | Root cause: the created tw row in `markets` makes `activeMarket` non-null, so the form requires a policy baseline for `<id>:country:service`; none existed (draft.marketID unchanged), so Save stayed disabled. Fix: SettingsWizard.tsx ~:1003 saveDraft with `policyObservation {target:<id>:country:service, version:0}` in the POST callback; marketID still deferred. |
| receipts / required | c | follow-on of the bundle:06 failure (same run) |

`retryPending` markets branch (SettingsWizard.tsx:809) already inserts the replayed market; unchanged.
Verification: NOT_RUN locally (owner rule); GREEN pending CI.

STATUS: DONE
