# Independent buyer payment PG gate author evidence — 2026-09-25

Scope: frozen `contracts/buyer-payment-http-v1.md` BPH01–04 only. Isolated branch
`codex/commerce-buyer-payment-pg-20260925`, contract baseline `603f237`;
dependency cherry-picks `7cd793c` (SQL/view) and `ea864d3`, `8c1260f`
(private HTTP/config). Test commits `d9b90e5` and `093cb00`. Delegation specified
`gpt-6-sol/high`; the runtime model was not independently exposed to this test.
No real provider, production endpoint, or persistent database was used.

Author verification:

| Gate | Result | Durable log / SHA-256 |
| --- | --- | --- |
| `go test ./tests/foundation -run '^$' -count=1` | exit 0, compile only | terminal readback |
| `bash scripts/dev/test-local.sh --payment` after view commit | exit 0, 4 new view top-level tests | `/Volumes/data/output/buyer-payment-view-author-1.log` · `3f87625979fac504015cf1f629b5ccd6e54fa8769c2b4985ab59e325dd1974c6` |
| first HTTP run | exit 1; disabled-mode *test request* omitted required prepare body/key and received 422 before disabled dispatch | `/Volumes/data/output/buyer-payment-http-author-1.log` · `0342450c5ee874bfdbc5ee0a0d57e062400ef9739e1df67a69a9da87fa16ca94` |
| corrected HTTP run | exit 0 | `/Volumes/data/output/buyer-payment-http-author-2.log` · `63d271a4053d1f1dbe0879b38697d3ada65bdb7f491dace2e05b08d63326012a` |
| expanded drift first run | exit 1; future proof fixture violated its own `observed_at < expires_at` check; reduced offset to one minute | `/Volumes/data/output/buyer-payment-http-author-3.log` · `687b9a150b29793692e1ae2e092f25296e690adc638a7f91536373fdd49e4b02` |
| final expanded BPH01–04 run, isolated PG18 with `-race -count=1` | exit 0; all 4 HTTP and 4 view top-level cases, including 22 candidate drift subcases | `/Volumes/data/output/buyer-payment-http-author-4.log` · `c62ace4c04fdf97616802fbc440c4e4387ed6785ad817c15e3516ea0b9988cb7` |

The script creates a task-owned disposable PostgreSQL container and removes it
on exit. Tests use synthetic PAYUNi credentials and signed local report fixtures;
no provider traffic. Test log assertions avoid serializing form/credential bytes.

Acceptance evidence:

- BPH01: original owned DRAFT amount and exact public JSON/method-name keys;
  no facts changed by read. Absent and foreign order map to Go not-found and
  actual HTTP 404. Candidate drift tests include market, current method head,
  enabled/visible, binding semantic version, credential rotation, qualification
  revoke/expiry/future/proof swap, hold expiry/generation, amount boundaries and
  whole-TWD rule. Binding asset and account provider/environment mutations were
  rejected by schema FK/CHECK constraints; they cannot be formed as live drift.
- BPH02: signed local capture path projects PENDING/AUTHORIZED/CAPTURED/REVIEW,
  with NONE/PREPARED/ISSUED/EXPIRED/config-mismatch page states and historical
  profile-switch `test_mode`/UNAVAILABLE; no read-side payment, job, stock,
  event, or receipt mutations.
- BPH03: actual private HTTP prepare→view→one-shot handoff→view/repeat for
  zh-CN, zh-TW and en, exact safe keys/original amount, independently decoded
  synthetic inner wire, no duplicate form or extra aggregate facts.
- BPH04: private auth/origin/capability/foreign/path/query/body/key/field/method
  denials, concurrent handoff exactly one ISSUED form, repeat no form, revoked
  capability and early closed-service 503 nonretryable, disabled feature 404.

Not claimed: BPH05 configuration assembly or BPH06 full repository race/vet and
independent source review, public BFF/browser flow, PSP sandbox or production.
Root agent owns those gates. Earlier failed logs are retained as test-fixture
corrections, not hidden or counted as passing gates.
