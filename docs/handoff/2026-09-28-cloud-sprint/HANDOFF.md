# Cloud sprint handoff — 2026-09-28 (paused for budget)

Branch `claude/gallant-bohr-9rs3yo`, draft PR luogangan7-lgtm/live_commerce_architecture_v1#1.
Paused by the owner (cloud credits). A new session continues from here. Read this file,
AGENTS.md, then the memory canvas element `agent:claude-ccr-live-commerce`.

## Landed on the branch (CI green)

GitHub CI `foundation` on `ce147fa` passed: 725 top-level tests, 0 FAIL, real PG18, race,
vet and govulncheck. Commits:

| Commit | What |
| --- | --- |
| c3878a4 | Replays return the same result regardless of host TZ (`command.InLocalTime`); test PG memory raised to 1g (Linux memcg OOM) |
| 2720f97 / ce147fa | CI timeout 60 min; govulncheck v1.8.0 (v1.1.4 panicked on Go 1.27) |
| 3d772e5 | Failure evidence written to repo `output/`, not the Mac-only `/Volumes/data` |
| 917cf46 (merged 465e08f) | Tests wait (fixed 5 s bound) for async PG backend teardown instead of snapshotting immediately; checkout `fetch-depth: 0` |
| 3b48d80 | Initial-store grant test compares the exact permission set |
| 5e273f7 | **T10 contract FROZEN**: `contracts/live-keyword-claims-v1.md` (see §0.1 integrator rulings) |
| 53f1c59 | Stripe PSP contract **DRAFT**: `contracts/stripe-psp-v1.md` (adversarial review was interrupted, not frozen) |

## Work in progress (NOT on the branch; saved as patches here)

Apply with `git am docs/handoff/2026-09-28-cloud-sprint/patches/<dir>/*.patch`, in this
order. Each was written by an isolated agent. None of it has been independently tested or
reviewed yet.

1. `t10-core/` — migration `0060_live_claims.sql`, `internal/claims` (+grammar kw-v1), `storefront.LockCartOwner`, vectors, and the author's own PG smoke `TestLiveClaimsCoreSmoke`. The author reported it green; that claim still needs verification.
2. `t10-ui/` (needs t10-core) — HTTP M1–M7 / B1–B2, the Studio Claims panel, storefront claim page, and the KC16 browser chain (`tests/foundation/browser_live_claims_test.go`, `tests/admin/claims-ui.spec.ts`). Committed by the ui worker before the pause. Run status is unknown.
3. `t10-tests/` (needs t10-core) — WIP independent PG gate tests KC02–KC15. **Interrupted, never run.**
4. `deploy-packaging-v1/` — `deploy/**` (Dockerfiles, compose, Caddy TLS, migration job, PG backup/PITR scripts, smoke) plus `docs/runbooks/{deploy,backup-restore,incident}.md`. Patches 1–3 were independently verified with a real docker build + compose smoke (verify-1 found failures F1–F3, which are now fixed, and the local smoke re-run passed). Patch 4 contains review-round edits that were **not re-verified**.

## Next steps (in order)

1. Apply t10-core. Then have an independent test_worker finish and run t10-tests: `bash scripts/dev/test-local.sh --live-claims`, or the focused `go test -run '^TestLiveClaims'` with a disposable PG. Fix product bugs at the root.
2. Apply t10-ui. Run `pnpm install --frozen-lockfile`, typecheck/build both apps, then `bash scripts/dev/test-local.sh --browser-live-claims` (Chromium at /opt/pw-browsers). Close the contract §0.1 P2 list (a)–(g), each with a test.
3. Independent security + correctness review of T10. Then a full suite and CI.
4. Apply deploy patches. Re-run the compose smoke (patch 4). Do a security/operability review of `deploy/**`.
5. Stripe: finish the adversarial review of `contracts/stripe-psp-v1.md` and freeze it with integrator rulings. The Stripe account's default currency is **HKD** (country HK); the contract must state store vs settlement currency. Then implement. Test tiers: MOCK + real sandbox + browser with card 4242.
6. T12 end-to-end loop, then minimal fulfilment/refund record, T20 security/fault injection, T21 review, T22 release.

## Credentials (values never in git or memory)

The owner uploaded Stripe sk_test and the Meta/Instagram/Threads app id and secret. In the
paused container they were written to `/root/.config/livecommerce/secrets.env` (chmod 600,
outside the repo). **A new container does not have that file.** Ask the owner to upload the
same file again, then regenerate it with the same variable names:
`STRIPE_SECRET_KEY, META_APP_ID/SECRET, INSTAGRAM_APP_ID/SECRET, THREADS_APP_ID/SECRET,
COMMERCE_META_APPS_JSON (page=Meta app, instagram=IG app, random verify_token),
COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID + COMMERCE_META_PAYLOAD_KEYS_JSON (random 32-byte key)`.

Read-only verification results:
- Stripe: acct_1UJDb0RusP6Wwj7e, livemode=false.
- Meta app 「大梦」: client_credentials OK.
- Threads: OK.
- Instagram Login app: cannot be checked with client_credentials; still unverified.

The owner should reset the Meta, IG and Threads app secrets after launch, because they
appeared in chat.

## Known risks / notes

- The full local suite on this 4-vCPU container takes about 25 min. The 90 s real-clock gates (MRR/LMR) are load-sensitive, so don't run heavy docker builds at the same time.
- `migrations.Apply` releases its lock only by closing the connection. A back-to-back Apply could, in theory, return busy. We could not reproduce this, so there is no product change.
- The pinned pre-LMR source 395b10d is reachable only through `origin/commerce/*` branches. Keep those branches.
- Migration numbers 0044–0059 are left for the local Codex lanes. This lane uses 0060 (T10) and 0061 (Stripe).
