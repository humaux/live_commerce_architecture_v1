# FLAKE-FIX delivery: #145 SU09 age-cutoff, #236 staff-team hydration

Role: test_worker (writer), sub-agent of the integrator. Model: Claude Opus 5.5. Base: r3/integration @ 8bf4e4f57.
Branch: fix/flake-su09-staffteam (worktree .claude/worktrees/flakefix). PR #243. Test-only change; no product code touched.

## #145 SU09 age-cutoff (REVIEW_REQUIRED instead of the cutoff text)

**Root cause: a fixture race between the age-cutoff action and the payment reconcile job. It is not a reload race and not a product bug.**
- `act age-cutoff` (tests/foundation/browser_stripe_test.go) moves the stored session's frozen deadlines back 36 min (`sstAge`, which bypasses guard_stripe_session) and ages the fake Stripe session. The live payment worker keeps running while it does.
- A query observation is recorded by `integration.record_stripe_observation` (post_river/0012), which checks `ExpiresAt` against the DB at record time. It is applied later, by its own `payment_reconcile_v1` job, through `apply_stripe_observation`. That function runs the I05 check (0167): if the report's `ExpiresAt` differs from `sess.expires_at`, it inserts a `PROVIDER_IDENTITY_MISMATCH` review case.
- Suppose an observation is recorded before `sstAge` and its reconcile job applies after `sstAge`. The order then becomes REVIEW_REQUIRED permanently, so the cutoff text never renders. No wait before the reload can help.
- Evidence: in the FAILSNAPs of occurrence 3 (PR #221, job 122207195018) and occurrence 4 (PR #237, job 122406035942), `reviews:1` already exists at O6. Only O7, the next step, creates reviews. The FAILSNAP state is REVIEW_REQUIRED.
- In production, `expires_at` is set once and frozen, so the product has no equivalent window.

**Change** (tests/foundation/browser_stripe_test.go only):
- New `sbEnv.quiesce`, called by `age-cutoff` before the aging. It stops the worker; with the worker stopped, nothing can record an observation. If a reconcile job for the attempt is still unfinished, it starts the worker, calls `sstWake`, drains the job (polling, at most 30 s), and stops the worker again. It makes at most 3 rounds, then the act fails with "N unapplied observation(s)".
- Then it ages both sides and restores the worker's prior running state.
- The brief suggested a facts poll before the reload. It was not used: the bad state is a durable review case, not a state that arrives late.
- Gate adapter tests/storefront/stripe-browser.mjs is unchanged. The 20 s wait, retries and assertions are untouched.

**RED/GREEN** (Gates workflow_dispatch, mode `--browser-payment`, probe on tmp branches only, never on the PR branch):
- The probe (tests/foundation/browser_stripe_probe145_test.go plus 2 hook lines) holds the attempt's next fake retrieve response for 3 s. While it is held, the worker is stopped gracefully, so the held query records its observation before the aging but its reconcile runs only after the aging.
- RED, base 8bf4e4f57 + probe: tmp/flake145-red3 018a27095, run 40577319893, job 122447872935. SU09_desktop and SU09_mobile both FAIL with the exact flake signature (`PROBE145 held=true pending_reconcile=1`, `reviews:1`, FAILSNAP state REVIEW_REQUIRED).
- GREEN, fix + same probe: tmp/flake145-green3 13e93e4a7 (fix eefb3ac80), run 40577325676, job 122447510586. SU09_desktop and SU09_mobile both PASS with `held=true pending_reconcile=1`.
- How the fix was found:
  - My first fix only stopped the worker around the aging. With a held-retrieve probe it reproduced the flake deterministically: tmp/flake145-green d62d35d9f, run 40568863451, job 122419957966, both viewports REVIEW_REQUIRED. That run is how the reconcile window was identified.
  - The same probe on the base passed (run 40568857208, job 122419651495), because a retrieve recorded after the aging is rejected at record time.
  - A lock-based probe (tmp/flake145-red2, run 40572471031, job 122430632536) did not reproduce on the base and was dropped.

## #236 staff-team en-mobile empty email

**Root cause:**
- The invite page's sign-up link is a plain `<a>` to `/signup#next=...`, so it is a full navigation followed by SSR and hydration.
- Playwright filled the SSR form before React hydrated, so the typed value never reached React state.
- PasswordAuth's mount effect `setNextPath(inviteNextFromHash(...))` changes state when a hash is present. Its re-render re-asserts the controlled `value=""`, so the form submits an empty email. The native "Please fill out this field" bubble appears and the code step never shows (staff-team.spec.ts:145).
- This explains why only the invite (hash) sign-up flaked.

**Change:**
- tests/admin/staff-team.spec.ts `signUp` and tests/admin/password-auth.spec.ts `fillCredentials` now wait until React has claimed the email input (`__reactProps$` on the node, via `expect.poll`) before filling, then assert `toHaveValue(email)`. No existing hydration marker exists; adding one would be a product change.
- The two direct reset-page fills in password-auth.spec.ts now go through `fillCredentials`.
- No other admin spec fills the auth forms (checked with grep).

**RED/GREEN** (mode `--browser-password-auth`). The probe in staff-team.spec.ts (tmp branches only) delays `/_next/static/chunks/` by 4 s on the invitee page before it clicks `invite-signup`.
- RED, base + probe: run 40568857208, job 122419651195. The staff chain fails at signUp, staff-team.spec.ts:145, with codeInput not found, which is the #236 signature. A repeat on tmp/flake145-red2 (run 40572471031, job 122430632573) failed the same way.
- GREEN, fix + probe: run 40568863451, job 122419957974. A repeat on tmp/flake145-green2 (run 40572476681, job 122434017892) passed. TestBrowserPasswordAuth also passed.

## Risks
- `__reactProps$` is a React-internal property name (React 19.3.0 here). If React renames it, the poll times out at 10 s, which is a loud failure, not a silent one. Upgrade path: a product-side hydration marker, which needs a ui owner.
- `quiesce` adds up to about 1–2 s to the age-cutoff action in normal runs. Its worst case is about 90 s before the act fails explicitly.
- The fix only covers `age-cutoff`. stripe_flow_test.go also calls `sstAge`, but without a live worker, so it is not exposed to this race.

## Evidence level and NOT_RUN
- E3 for the probe RED→GREEN runs above. The final PR head CI is reported on PR #243.
- Probe branches tmp/flake145-{red,green,red2,green2,red3,green3} are pushed for evidence. Delete them after review.
- NOT_RUN locally, as the brief requires CI only. Local checks were gofmt, `go vet -tags browser ./tests/foundation/`, and tsc on both specs (the only errors were the missing node types in this environment).
- NOT_RUN: the WebKit run (`--browser-webkit`), the full browser universe, and a flake-rate measurement over N repeated runs.
- Files outside the brief's allowed list: output/flake-fix/DELIVERY.md (this file).
