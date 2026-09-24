# Published storefront resolver — bounded local acceptance

Status: PASS_BOUNDED_INTERNAL_PUBLISHED_ORIGIN_RESOLVER.
Date: 2026-09-25 Asia/Shanghai (runs on 2026-09-24 UTC).
Code baseline: `cbffd8a`; contract freeze `f153026`.

## Delivered boundary

[Contract](../../contracts/published-storefront-resolver-v1.md): a no-cache exact
HTTPS-origin lookup requiring independent publication consent, active tenant/store,
ACTIVE domain and finite, currently valid ownership/TLS facts. No automatic
publication of existing stores. Migration0020 adds two private FORCE-RLS tables
and one fixed SECURITY DEFINER lookup; only the checked buyer issuer may execute
it. No application role can provision these control-plane facts.

The Go resolver returns five internal routing fields only, checks returned shape,
uses the existing UUID validator/authority check, and sanitizes database failures.
Domain resolution is request admission, not buyer authorization. A request admitted
before a committed suspension may finish; later requests must resolve again and
deny. A synthetic domain remap does not make an old-store buyer capability valid.

This is NOT a DNS/TLS verifier, domain lifecycle/publication writer, storefront
HTTP handler, public checkout, or production readiness result. All domain proof
facts in tests are explicit **SYNTHETIC_LOCAL_TEST_NOT_PROVIDER_PROOF** fixtures.

## Actual gates

| Evidence | Result |
| --- | --- |
| Root `go test -race -count=1 ./internal/domains ./internal/platform` | exit0; domains2.903s, platform2.704s |
| `bash scripts/dev/test-local.sh --storefront-resolver` | exit0; 5 top-level PASS, 0 FAIL, 0 SKIP; foundation4.315s |
| `bash scripts/dev/test-local.sh` | exit0; 317 top-level PASS, 0 FAIL, 0 SKIP; foundation155.344s; race and `go vet ./...` included |
| `python3 scripts/check_packet.py` | PASS_PACKET_STRUCTURE_ONLY; not application or production acceptance |
| `bash -n scripts/dev/test-local.sh`, `git diff --check` | exit0 |
| Fixture postflight | `docker ps -a --filter label=livecommerce.fixture` returned no rows |

Logs, retained on the data volume:

- `/Volumes/data/output/live-commerce-published-resolver-tests/targeted-initial.log`
  SHA256 `80a4cb86a24d1c37417ad3cb8efad036c633872c590e22782d3cb83f62da47da`.
- `/Volumes/data/output/live-commerce-published-resolver-tests/full-initial.log`
  SHA256 `a539b148e795632687ea3b0482e8a82d3573e94361d1b6f8db178a70bb9952fd`.

PR01–PR05 cases: Go/direct-SQL grammar parity including label/total length limits,
ports, controls, Unicode and local suffixes; active-without-published denial;
independent domain state, tenant/store activity, publication, future proof and
expiry denials; finite timestamp/required proof constraints; exact role/ACL and
constructor rejection, table/column denial, FORCE RLS, tenant/store FK and global
origin uniqueness; committed stop/renewal readback; no session issuance from lookup;
cross-store buyer denial after synthetic remap; deadline/closed-pool sanitization.

The expiry-after-wait test first succeeds with a short-lived proof, obtains an
actual table lock, observes the resolver's `pg_stat_activity` Lock wait, waits
until the **database clock** says the proof expired, releases the lock, and
requires denial. It does not infer concurrency merely from a goroutine/sleep.

## Review and coordination

- Read-only inventory: `buyer_entry_inventory`, gpt-6-luna/medium, baseline
  `1baa0d2`, no source writes; Humaux `112e376d-11b3-4c46-bbcb-e56535b8f091`.
- Independent preflight: `published_resolver_review`, gpt-6-astra/high,
  no source writes; bounded PASS at `cb1e1c2`, Humaux
  `78afbe9d-1fd6-48b0-a304-88230a13bd5d`.
- Go author: `published_resolver_go`, gpt-6-sol/high, commerce_worker;
  base `f153026`, branch `codex/published-resolver-go-20260925`, worktree
  `/Volumes/data/live-commerce-worktrees/published-resolver-go-20260925`.
  Only `internal/domains/**` plus the one new platform validator wrapper.
  Author `044702f` + `c8d66ae` merged as `6a956b4` + `acbdf57`.
- Root integrator: migration0020, five real-PG tests, focused existing-runner mode,
  contract refinement and independent execution, commit `cbffd8a`.
- Review refinements closed before the runs: control/blank evidence rejection,
  observed-lock final-clock test, and reuse of `command.ValidID` instead of a
  duplicate UUID parser. No failed run or removed test was needed in this slice.
- Code graph: 5 files submitted under `live-commerce`, 4 Go files/80 entities
  indexed; SQL migration is validated by real PG, not a claimed codegraph parse.
  Validator and expiry test linked to review decisions.
- Final independent review: `d63a2a8b-a9da-419c-b56a-f27faba1c97a`, bounded PASS
  at `cbffd8a`, no remaining P0/P1; both P2 refinements closed. Reviewer independently
  ran the Go race packages and checked both root PG logs/hashes, but did not rerun
  the PG suite. PR06 is satisfied for this local prerequisite only.

## Still required

Trusted, audited ownership/TLS/publication writing; complete domain lifecycle and
custom-domain deployment; buyer cookie/CSRF/rate-limit and HTTP composition;
allowlisted public DTOs (never raw internal checkout snapshots); buyer three-locale
visual design/UI and browser tests; real provider eligibility, production workers,
and the rest of the full SaaS gates. No existing customer service, live stream,
order, payment or configuration was changed. No UI/browser code changed in this
slice; prior browser acceptance is not relabeled as a new buyer-browser test.

Maintenance dependency rationale: [dependencies](dependencies.md).
