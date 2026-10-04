# Unit ops-disk-guard: bounded WAL archive, reachable alerts, bounded Docker growth (P0 incident fix)

## Incident (2026-10-03/04, pilot 35.212.187.34)

- **What happened.** The root disk filled to 96G of 96G.
  - `/var/backups/live-commerce/wal` held 79G: 5051 archived 16MB segments.
  - At 2026-10-03 06:40 UTC PostgreSQL failed a checkpoint write ("No space left on device") and shut down.
  - Docker's restart could not create the overlay mount, so **the DB stayed down for about 29.5 hours**.
  - Nightly dumps and the weekly base backup also failed, because their containers could not start.
  - The api `/healthz` kept answering 200. The containers' `/readyz`-based health checks were unhealthy.
- **Nobody was alerted.** The watchdog's W1, W2, W3 and W4 checks all failed, but alerts only reach cron `MAILTO=root`, and `LC_ALERT_WEBHOOK_URL` is unset.
- **Recovery** (owner-approved, logged here):
  1. `docker builder prune` freed 4.7G.
  2. `docker start` on the postgres container; crash recovery completed.
  3. `pg-ops.sh basebackup` produced `20261004T120907Z`.
  4. `pg_archivecleanup /backup/wal 0000000100000013000000C9.00000028.backup` brought disk use to 14%.
  5. `pg-ops.sh backup --tag post-incident` succeeded. The watchdog passes again.

## Root causes (all are code or configuration in this repo)

1. **WAL volume.** `deploy/postgres/postgresql.conf` sets `archive_timeout = 60s`. Every minute this forces a full 16MB segment, even on an idle DB: about 23GB a day. `archive_command` copies segments uncompressed.
2. **Retention window versus volume.** pg-ops keeps 2 base backups and all WAL since the older one. Base backups are weekly (cron `23 3 * * 0`), so the window is up to 14 days, roughly 320GB at that rate. The pruning logic exists; the sizing is wrong. This violates invariant **I23**: bounded growth.
3. **Alerts never reached a human.** There is no owner-visible channel.
4. **Docker build cache and old images grow** with every host build, by about 6.5G over 4 builds. Nothing prunes them.

## Decisions

- **D1 Compressed archive.** WAL segment files are archived gzip-compressed as `%f.gz`. A forced-switch segment is mostly zeros and compresses to kilobytes.
  - These files stay plain copies: `.history`, `.backup` and `.partial`.
  - Idempotency is kept: if the target already exists, compare the decompressed content with `%p`.
  - The write stays atomic: `.part`, then sync, then mv.
  - `archive_timeout` stays at **60s**, so the one-minute RPO is unchanged.
- **D2 Restore reads both forms.** `restore_command` and `restore-pitr.sh` use `.gz` when present and fall back to the legacy plain file. Pruning (pg-ops `basebackup` retention, and any `pg_archivecleanup` use) removes both forms, using `-x .gz` plus the plain pass.
- **D3 Daily base backup.** Change the cron from weekly to daily, at the same minute; keep `kept_bases=2`. The retention window becomes 2 days or less. The DB is 46MB and its base backup is 6.6MB, taking 2 seconds. Update W3's base-age threshold to match: newest base older than 26h fails.
- **D4 Alerts reach the owner by e-mail.**
  - **What is sent.** When any check fails, `watchdog.sh` sends one e-mail through the existing app SMTP relay to `LC_ALERT_EMAIL`. The pilot value is `ailun@xgdwm.com`, which forwards to the owner.
    - The relay comes from `COMMERCE_SMTP_HOST`, `COMMERCE_SMTP_PORT`, `COMMERCE_SMTP_USERNAME`, `COMMERCE_MAIL_FROM` and the `commerce_smtp_password` secret.
    - Sending uses `curl smtps`/`smtp` with STARTTLS as the port requires.
  - **When it is sent.**
    - A changed failing set sends at once.
    - An unchanged failing set re-sends at most every 6 hours.
    - A transition back to all-PASS sends one "recovered" mail.
    - State lives in the existing watchdog state file.
  - **Secret handling.** The password must never appear in argv (`ps`), logs or the state file. Pass it via `curl -K -` from stdin, or a 0600 temp netrc that is removed in a trap.
  - **Failure handling.** A mail failure is logged, never fatal, and never retried in a tight loop.
  - **Coexistence.** `LC_ALERT_WEBHOOK_URL` keeps working alongside e-mail.
  - **Preflight.** It WARNs when neither `LC_ALERT_EMAIL` nor `LC_ALERT_WEBHOOK_URL` is set, and FAILs that case in production.
- **D5 Bounded Docker growth.** After a successful build, `build-images.sh` runs:
  - `docker builder prune -f --keep-storage 3GB`;
  - removal of `lc-*` images whose tag is neither the running `IMAGE_TAG` nor the previous deployed tag in deployments.log. The previous tag must survive so `app-rollback` still works.
- **D6 Disk budget guard before writes.** `pg-ops.sh backup` and `basebackup`, and `deploy.sh upgrade` before its forced backup, refuse to start with a clear message when the backup filesystem has less than 10% or 2G free, whichever is larger. A full disk must fail loudly before the DB is touched.
- **D7 Runbook.**
  - `docs/runbooks/incident.md` gains a "disk full / WAL archive" section containing exactly the recovery above.
  - `deploy.md` §4.2 gains:
    - `LC_ALERT_EMAIL`;
    - the cron change, applied from the crontab example during the upgrade;
    - an external uptime monitor recommendation on `https://api.<domain>/readyz`, not `/healthz`. First verify that `/readyz` is reachable through Caddy and returns 503 when the DB is down. If it is not exposed, say so; do not expose new routes without a ruling.

## Out of scope (owner decisions, recommended in the runbook)

- A separate persistent disk for `/var/backups`.
- An off-host copy of the nightly dumps, for example a GCS bucket.

## Gates (red first where possible; label MOCK/REAL)

- **OD1 (REAL PG, local Docker).** Start the repo's postgres image and config with an archive dir.
  - Idle it for at least 5 minutes with `archive_timeout=60s`, and force a few switches.
  - Assert: `.gz` segments; average archived bytes per segment under 200KB; a projected daily volume printed; idempotent re-archive of the same segment (exit 0, no duplicate); a corrupted existing target is detected (non-zero).
- **OD2 (REAL PG).** PITR from a base backup plus compressed WAL, plus a mix of legacy plain segments, restores a row inserted after the base. Use `restore-pitr.sh --drill` if it runs locally; otherwise give an equivalent scratch-container test, and say which.
- **OD3.** pg-ops retention prunes both `.gz` and plain segments older than the oldest kept base. Nothing newer is touched.
- **OD4 (MOCK SMTP).**
  - The watchdog sends exactly one mail per failing-set change, re-sends after 6h (with an injected clock or state), and sends one "recovered" mail.
  - The password appears in no argv captured during the run, in no log and in no state file.
  - The SMTP-down case is non-fatal.
- **OD5.** Disk guard: with free space below the threshold (a small loop-mounted or tmpfs fs, or an injected `df`), backup, basebackup and upgrade refuse before touching PG.
- **OD6.** build-images prune keeps the current and previous tags. Run it statically, or with fake images in local Docker.
- **OD7.**
  - `bash -n` and shellcheck on the touched scripts;
  - `scripts/dev/check-gates.sh`;
  - smoke static (`deploy/scripts/smoke.sh` static mode);
  - existing deploy/backup tests stay green;
  - full G07 is not required unless Go or SQL changes.

## Delivery

- Branch `fix/ops-disk-guard`, worktree `.worktrees/ops-disk-guard`, base `f63153ad`. Only deploy, runbook and test paths change.
- Deliver `output/ops-disk-guard/SUMMARY.md` with commands, exit codes, evidence and NOT_RUN items.
- No push. No access to the pilot host; the integrator deploys.
