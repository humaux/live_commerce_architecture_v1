# RELAY 4 — PR #403 head 6273d872 (run 41233820368)

Failing: gate (bundle:06) -> --browser-identity -> TestBrowserSettingsWizardRealChain; receipts/required follow. All other jobs passed (relay3's :270 fix held: that line did not fail this time).
Playwright log (artifact gate-browser-identity, settings-real-*/playwright.log): `settings-v2.spec.ts:1085` marketSelectionAcceptance, `Save pricing policy` click timed out, button `disabled`.

| finding | class | change |
|---|---|---|
| settings-v2.spec.ts:1085 Save disabled on a later market-selection case | b (test race, not product: the loop closes each page as soon as the route handler sees the PUT; SettingsWizard.tsx:734/759 keeps the command journal in localStorage until the reply is processed, localStorage is shared by every page of the context, so the next case loads with `pending` set and Save disabled. Same product code passed this step at 40b599fe, so timing-dependent) | tests/admin/settings-v2.spec.ts:1094-1097 poll until no `commerce-settings-command:*` key remains before the page closes (measurement only, no assertion removed or loosened) |
| RVFIX2-403 product fix | not implicated | none |

Not run locally (owner rule). GREEN run id: pending CI on the next push. Unproven: the failing case name was not in the log; the race explanation fits the code and the earlier pass of the same product code.

STATUS: DONE
