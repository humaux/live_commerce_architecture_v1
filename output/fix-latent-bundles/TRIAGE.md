# FIX-LATENT-BUNDLES — round 1 trigger

Date: 2026-11-01. Base: `4b8c04c7213025c173e260db6f9c81cb5b818c61`.
Branch: `unit/fix-latent-bundles`; worktree: `.worktrees/fix-latent-bundles`.
Role: dispatched Codex implementation lane; exact runtime model/reasoning are not exposed.
Authority: integrator's round 1 card, repository AGENTS.md and 架构.md §§0, 1, 2, 26, 27.

## Scope and selection

Only two header-comment lines change executable source; product and test logic remain identical to base.
`tests/foundation/browser_taiwan_cvs_test.go` owns `TestBrowserTaiwanCvs/WebKit` and selects `--browser-cvs`.
`tests/admin/inbox-ui.spec.ts` owns INU05 and is an input of `--browser-inbox`.
These relationships are recorded in `output/ci-select-backend-browser/browser-ownership.json`, regenerated with the existing script.
The CVS failure occurs in the buyer half before the merchant spec, so its owning Go harness is the precise trigger.
Allowed writes: those two comment headers, the generated ownership file, and this report directory.

## Historical CI failure lines (E3 for the referenced historical SHAs only)

Read on 2026-11-01 with `gh run view <run> --job <job> --log-failed`, filtered to failure coordinates and counts.
No downloaded artifacts are committed. These historical failures do not establish a result for this trigger's HEAD.

### CVS — PR #396, Gates run 41275260978, job 124756873741 (bundle:07)

Source: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41275260978/job/124756873741
Logged SHA: `88dbb8d711b3f684e266e89b287dfb524fbfec74`.

```text
2026-11-01T05:29:12.3374954Z browser_taiwan_cvs_test.go:159: storefront browser gate failed: exit status 1
2026-11-01T05:29:12.3870913Z --- FAIL: TestBrowserTaiwanCvs/WebKit (50.78s)
2026-11-01T05:29:12.6475426Z bundle member --browser-cvs: failure
```

The first excerpt retains the failure text and omits only its runner-local evidence path.
MOCK passed; SANDBOX was skipped for absent external sandbox configuration.

### CVS — PR #440, Gates run 41278013182, job 124764811243 (bundle:07)

Source: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41278013182/job/124764811243
Logged SHA: `c2ac3ab67f10de7027c916b5d7fb5bbcf913220a`.

```text
2026-11-01T06:21:58.5709293Z browser_taiwan_cvs_test.go:159: storefront browser gate failed: exit status 1
2026-11-01T06:21:58.6041238Z --- FAIL: TestBrowserTaiwanCvs/WebKit (93.43s)
2026-11-01T06:21:58.8924145Z bundle member --browser-cvs: failure
```

The first excerpt omits only its runner-local evidence path. MOCK passed; SANDBOX skipped.
Exact inner Playwright assertion, expected/actual values, and root cause: UNKNOWN; the job log reports the Go harness assertion only.

### Inbox — PR #440, Gates run 41278013182, job 124764810973 (bundle:06)

Source: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41278013182/job/124764810973
Logged SHA: `c2ac3ab67f10de7027c916b5d7fb5bbcf913220a`.

```text
2026-11-01T06:07:17.2147936Z browser_inbox_ui_test.go:358: Playwright 1 failed
2026-11-01T06:07:17.2149631Z browser_inbox_ui_test.go:358: Playwright 87 passed
2026-11-01T06:07:17.2151409Z browser_inbox_ui_test.go:358: Playwright failed INU05 (inbox-ui.spec.ts:472:1)
2026-11-01T06:07:21.5882901Z browser_inbox_ui_test.go:402: inbox browser failed: exit status 1
2026-11-01T06:07:28.9232805Z bundle member --browser-inbox: failure
```

The harness failure excerpt omits only its runner-local evidence path.
Its header invariant explicitly limits logs to INU IDs, source coordinates and counts.
Exact inner assertion and expected/actual values: UNKNOWN from this log; INU05's test-start coordinate is not an assertion coordinate.
Do not infer that the hidden-thread assertion specifically failed merely from the INU05 case name.

## Verification and handoff

Run only the existing static ownership generator and its `--check`; no local tests or servers.
Static results: `node scripts/dev/derive-browser-ownership.mjs` exit 0 (55 modes);
`node scripts/dev/derive-browser-ownership.mjs --check` exit 0; `git diff --check` exit 0.
The generated diff changes exactly the two edited files' source hashes; mode membership is unchanged.
This is the tests-only trigger stage: no implementation or GREEN claim in round 1.
Integrator opens the PR into `r3/integration` after its guard and records that PR's Gates/Foundation run IDs.
Acceptance: inspect this trigger's own CI to establish RED or GREEN on unchanged base logic.
NOT_RUN: local tests, local servers, browser execution, SANDBOX/LIVE, independent review and this trigger's PR CI (pending integrator PR creation).
Own PASS claims are E0; successful static checks establish E1 only. No product fix is claimed.
No temporary downloads, fixtures, servers or background processes were created.
