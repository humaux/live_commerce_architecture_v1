# T07 Meta durable admission — implementation and actual gates

2026-09-26. **PASS_LOCAL_MI01_07_NOT_PUBLIC**, not public/provider or
full-SaaS acceptance. Frozen interface: [Meta inbox v1](../../contracts/meta-inbox-v1.md).
Product source baseline for the final full-tree run: `8dd693a`.

## What changed

- `0028_meta_inbox.sql`: immutable cross-app asset ownership, registrar-controlled
  routes, transaction-owned receipts/events and ordinal accounting; separate
  raw/scoped/quarantine ciphertext; deferred completion guards and audited
  terminal retention. Merchant binding self-assertion never establishes routing.
- `post_river/0003_meta_inbox_queue.sql`: reserve kind `meta_inbox_v1` and queue
  `meta_inbox` for the dedicated producer, check both INSERT and identity UPDATE,
  and require one completed routed event for each inserted job at COMMIT.
- `meta.NewInbox` / `NewInboxHandler`: exact verified raw bytes and normalized
  payloads use the previously accepted AEAD; one READ COMMITTED transaction
  inserts ciphertext and River jobs; bounded rollback and safe 503 on failure.
  Historical body/event identity precedes mutable routing; no worker is started.
- `platform.ValidateMetaIngressPool`: borrowed-pool startup validation rejects
  privileged, mixed, SET-capable and predefined-system-role logins. The SQL
  functions repeat authority checks; no caller-controlled tenant GUC is trusted.

No dependencies were added. Call paths and upgrade tests are recorded in
[dependencies](dependencies.md). Site chat, Meta messaging and identity/consent
domains remain separate. No production/customer system or live broadcast changed.

## Tests and evidence

| Gate | Concrete evidence |
| --- | --- |
| MI01 | Real dedicated roles, mixed-role and SET negatives, exact provider binding, immutable foreign-store/cross-app owner, predefined-role admission refusal |
| MI02 | Independent stdlib AES-GCM recovery of exact raw bytes and scoped/quarantine payloads; explicit AAD; private SELECT denial; job args contain only internal ID/version |
| MI03 | True mixed tenant A/B/unknown batch; fault triggers at eight persistent stages leave every storage count unchanged; retry succeeds; incomplete batch COMMIT rejected |
| MI04 | Concurrent identical body and rebatched MID dedupe; changed payload conflict; post-revoke/reauthorization/purge/job-prune replay does not resurrect or rehome |
| MI05 | Observed real binding lock wait crosses proof expiry; finalized event expires before COMMIT; inactive tenant/store and stale binding version cannot route |
| MI06 | Signed handler plus PG commits before ACK; injected deferred failure returns fixed 503; failed response write after COMMIT then replay adds nothing |
| MI07 | Age alone cannot purge; explicit curator evidence plus terminal/pruned job required; locked job skips promptly; raw waits for every member; permanent metadata survives |

Root targeted run at `ee41227`: `bash scripts/dev/test-local.sh --meta-inbox`,
exit 0, 17 top-level tests, foundation 5.202s.
`/Volumes/data/output/meta-inbox-root-final-target-1.log`, SHA-256
`7376367ff4eef15428264cb0af62ef0f29afe6e8df365edf02bc76d80b9f5be6`.
An additional explicit migration-owner regression was added in `8dd693a`.
Final full run at that product/test baseline: `bash scripts/dev/test-local.sh`,
exit 0, **507 top-level PASS, 0 FAIL, 0 SKIP**, real isolated PG18, race and
`go vet ./...`; foundation package 310.335s.
`/Volumes/data/output/meta-inbox-root-full-final-2.log`, SHA-256
`308a90858a105d731fa6d79a8016b8e67136eb96585cdac7a3772ee61a34d7b3`.

Then test-only Instagram supplement `9dcc436` integrated as `71a8e1f` adds
comment/live_comment/message routing, provider mismatch, object-bound AAD and
Page-route isolation. Product `internal/`, `migrations/` and `post_river/`
source is unchanged from the full-run baseline. Root reran `--meta-inbox`:
**20 top-level PASS, 0 FAIL, 0 SKIP**, exit 0, foundation 6.424s; final
`go vet ./...` exit 0. This is a separate gate, not a claimed 509-test full run.
`/Volumes/data/output/meta-inbox-root-instagram-final.log`, SHA-256
`a861e32a65e7f09bbce6a2a358e65236b6d8a8b5d9ccb2b44bbae47ebdc0d7b8`.
Independent final IG runner also passed 20 tests in 5.320s, with vet exit 0:
`/Volumes/data/output/meta-inbox-pg-instagram-first-20260926.log`.
Owned fixture containers were removed by the runner; postflight found no
`livecommerce.fixture` containers. Evidence and author worktrees are retained.
Documentation checks: 90 relative links exist, `git diff --check` exits 0,
and the packet validator returns `PASS_PACKET_STRUCTURE_ONLY`; that last
check is not a substitute for the executable gates above.

Independent Go author `integration_worker` (gpt-6-sol/high), branch/worktree
`commerce/meta-inbox-go-20260926` / `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`,
base `80064b3`, own commit `a6c41aa` integrated as `fb83695`; allowed paths were
`inbox.go`, `inbox_test.go` and its [author note](2026-09-26-meta-inbox-go-author.md).
Independent PG test worker used `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`
from the same base; commits `ddf09cc`/`9d7cc99`/`9dcc436`, isolated test paths only.
Its actual model/reasoning identifiers were inherited, not exposed by the tool.
See [test author note](2026-09-26-meta-inbox-pg-author.md); initial combined run
15 tests, 3.936s, exit 0, `/Volumes/data/output/meta-inbox-pg-combined-20260926.log`.
Root separately reviewed tests, added queue/authority, eight-stage rollback,
mixed-tenant, response-loss, final-proof and disabled-scope checks, and reran them.

## Defects found and fixed, without discarding failures

1. `c23ca69`: ordinary job-state SELECT could race a lifecycle update while
   purging ciphertext. Final `FOR SHARE SKIP LOCKED` job checks now hold locks
   through DELETE, including all raw-batch members. Independent causal two-TX
   regression passes; an executable old-source RED control was **NOT_RUN**.
2. `c49f025`: predefined PostgreSQL roles confer extra authority without owning
   tables or having BYPASSRLS. Both startup and SQL refuse these mixed logins.
   See [PostgreSQL 18 predefined roles](https://www.postgresql.org/docs/18/predefined-roles.html).
   This refuses unsafe provisioning; it does not revoke direct SQL rights an
   administrator wrongly granted. Such credentials require grant remediation.
3. `0059521`: `pg_has_role` reports implicit membership for superusers. The new
   generic-job guard initially misclassified migration-owner jobs as ingress,
   breaking five old queue suites. Exclude superusers only from that membership
   branch; every literal Meta kind/queue still reaches the strict authority check.
   Existing tests were not weakened. Original full run exit 1, foundation 301.346s:
   `/Volumes/data/output/meta-inbox-root-full-1.log`, SHA-256
   `caf3549a121fedbafe37d2fd2236b7e7a7e9b4902a53039d035d2cf216bee1bd`.
   Fixed expiry subset exit 0, 53.672s:
   `/Volumes/data/output/meta-inbox-root-expiry-regression-1.log`, SHA-256
   `55e048d9f9bc8094c50074918ce8866c0c9edb385d71ac1a84dc2cbcd95b6d73`.

Read-only source review found no unresolved P0/P1/P2 after these fixes and the
explicit read-committed change. Review records:
`065cb2fa-a007-405e-a676-3d2cdb4a190a` and
`4170a3cf-5aa3-44eb-ae44-b1c113ca66c3`. Independent actual PG/IG records:
`55982e60-774d-481a-8f1e-1a8caecf0001` and
`95e30058-260a-4325-97bf-ad1c18d65039`. The separate final source and runtime
evidence above define the bounded acceptance, not the source verdict alone.
Final independent source/evidence adjudication `28f3b739-2f36-4632-bc1f-dd40ad10757d`
accepts MI01–07 locally with no unresolved P0/P1/P2, explicitly excluding the
consumer, OAuth, public/provider operation and full SaaS delivery.

## Still required before public mount / full delivery

Verified OAuth proof issuance and route activation workflow; scoped consumer
processing into the separate Meta conversation/comment domain; terminal-event
no-op/cancel handling; authorized review UX; production key loading/rotation;
retention scheduling, quotas and backlog alerts; deployment/recovery tests and
real Meta qualification. Registrar/curator remain unassigned outside synthetic
fixtures. This increment adds no UI and claims no new browser acceptance.
The whole SaaS also retains its checkout/logistics/other-module release gates.
