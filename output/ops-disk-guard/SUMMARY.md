# ops-disk-guard — P0 pilot disk-full root fix

This file was written by the integrator. The implementer was a Sonnet agent, and the harness refused its report file.

- **Implementation:** commits `1be9b03e`..`50422b4b` on `fix/ops-disk-guard`, merged into r3/integration as `bfe9bf22`.
- **Brief:** `docs/delivery/units/ops-disk-guard.md`.

## Root causes and fixes

| Cause | Fix |
|---|---|
| `archive_timeout=60s` with uncompressed 16MB segments: about 23–24 GB/day | D1: `deploy/postgres/archive-wal.sh` gzips segments. It reads back, compares, syncs, renames atomically and stays idempotent. `.history`, `.backup` and `.partial` files stay plain. RPO stays at 60s. |
| Weekly base backups, so the retention window was up to 14 days | D3: the base backup runs daily (`23 3 * * *`) with `kept_bases=2`, so the window is 2 days or less. W3's base-age threshold is 26h. |
| Pruning ignored compressed files | D2: `pg_archivecleanup -b -x .gz` prunes both the compressed and the plain form. `ops/restore-wal.sh` restores from either. |
| Alerts went only to `MAILTO=root` | D4: `watchdog.sh` e-mails `LC_ALERT_EMAIL` through the existing SMTP relay. The password goes through curl `-K` from a process substitution, so it never appears in argv or logs. Mail goes out on a failing-set change, again after 6h if unchanged, and once as RECOVERED. `--test-mail` is available. Preflight requires an alert channel. |
| Docker build cache grew without bound | D5: `prune-docker.sh` runs after `build-images.sh`. It applies `builder prune --keep-storage 3GB` and keeps the current and previous `lc-*` tags, so app-rollback still works. |
| A full disk took the DB down | D6: `lc_disk_guard` refuses `pg-ops backup` and `basebackup`, and `deploy.sh upgrade` before its backup, when free space is below max(10%, 2GiB). |
| No runbook | D7: incident.md gains a "disk full / WAL archive" section; deploy.md §4.2 and backup-restore.md are updated. |

## Measured (REAL PG 18.6, repo conf, idle with forced switches)

| | Bytes per segment | Per day |
|---|---|---|
| Before | 16,777,216 | 24.16 GB |
| After | about 31.5 KB average (steady state about 16.5 KB) | 0.045 GB (about 530× less) |

Real traffic compresses less. W2 (disk above 80%) now reaches the owner.

## Gates

- **Implementer:** `bash scripts/dev/test-local.sh --ops-disk-guard`, 96 PASS, exit 0.
  - OD1–OD3 ran on REAL PG: compression, idempotency, corruption detection, PITR `--drill` from base plus `.gz` plus legacy plain WAL, and pruning of both forms.
  - OD4: mock SMTP, with a stub and a real curl against a local STARTTLS relay.
  - OD5: disk guard, with stubbed df and a real tmpfs.
  - OD6: prune keeps the current and previous tags.
  - Negative controls against the pre-fix files fail as expected.
- **Integrator rerun (non-author):** 99 PASS, exit 0 (`output/ops-disk-guard-review/rerun.log`).
- **On r3/integration after the merge:** `check-gates` ok (63 modes), `test-node` exit 0.

## NOT_RUN

- shellcheck locally (not installed here; CI deploy-smoke S01 runs it).
- `smoke.sh full` (Linux root): CI deploy-smoke.
- The real SMTP relay and the real host cron: pilot rollout steps 3–4 below.

## Findings

- `/readyz` checks the DB, but it is not routed through Caddy. The api host is default-deny and only routes `/healthz`, which always returns 200. An external uptime monitor therefore needs a ruling that exposes `/readyz` (status only).
- Rollback limitation: the pre-fix `restore-pitr.sh` cannot read `.gz` segments. This is a forward-only change.

## Pilot rollout (with the R5 upgrade, per runbook §4.2)

1. Add `LC_ALERT_EMAIL=ailun@xgdwm.com` to compose.env. Confirm that `LC_SMTP_HOST`, `LC_SMTP_USERNAME`, `LC_MAIL_FROM` and the `commerce_smtp_password` secret are set.
2. Run preflight, then `watchdog.sh --test-mail`; the owner confirms the mail arrived.
3. Copy `deploy/host/crontab.example` to `/etc/cron.d/live-commerce`, which switches the base backup to daily.
4. Run `build-images.sh`, which now prunes after the build, then `deploy.sh upgrade <sha12>`. The postgres container is recreated for the new bind mount, a few seconds of downtime.
5. Verify:
   - the watchdog passes;
   - `SHOW archive_command` shows the new script;
   - a forced switch writes a `.gz` segment;
   - `failed_count` stays flat.
6. Run `pg-ops.sh basebackup` once, which prunes the legacy plain segments.
