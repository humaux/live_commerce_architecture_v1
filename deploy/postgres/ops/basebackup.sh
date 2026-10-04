#!/usr/bin/env bash
# File: deploy/postgres/ops/basebackup.sh
# Purpose: daily physical base backup for PITR (deploy-design §15): pg_basebackup in
#   compressed tar format with streamed WAL and a SHA256 manifest, verified with
#   pg_verifybackup, then keep the newest N bases and prune archived WAL older than the
#   oldest kept base (pg_archivecleanup to its START WAL segment from backup_label). Daily
#   (not weekly) keeps the WAL retention window at <= 2 days: the pilot's weekly bases kept up to
#   14 days of WAL, 79 GB at 16 MB/segment, and filled the disk (2026-10-03, invariant I23).
# Runs as/in: pg-ops container (UID 999, socket only) via `deploy/scripts/pg-ops.sh basebackup`;
#   cron daily 03:23 (deploy/host/crontab.example).
# Reads env: LC_BASE_KEEP (default 2).
# Reads secrets: pg_superuser_password (pgpass on tmpfs; pg_hba allows `local replication postgres`).
# Output: /backup/base/<UTC>/{base.tar.gz,pg_wal.tar.gz,backup_manifest}; prunes /backup/wal (both the
#   compressed <seg>.gz and legacy plain segments, plus backup-history labels, older than the oldest kept base).
# Used by: restore-pitr.sh, watchdog W3, docs/runbooks/backup-restore.md.
# Depends on: ops/lib.sh; archive_command writing /backup/wal (postgresql.conf, postgres/archive-wal.sh).
# Status: DESIGN; verified by smoke S30 (V3: pg_verifybackup on PG 18 tar format; WAL parsing
#   is skipped with -n because WAL inside a tar cannot be parsed — WAL completeness is proven
#   by the PITR drill S31 instead).
# Change rules: never prune WAL before the new base verified; keep >= 2 bases.
export OPS_SCRIPT=basebackup
# shellcheck source=deploy/postgres/ops/lib.sh
source /ops/lib.sh

keep=${LC_BASE_KEEP:-2}
[[ "$keep" =~ ^[1-9][0-9]?$ ]] || ops_die "invalid LC_BASE_KEEP" 2
((keep >= 2)) || ops_die "LC_BASE_KEEP must be >= 2" 2
ops_auth
run_id=$(ops_utc)
start=$(date +%s)
mkdir -p "$OPS_BACKUP/base"
work="$OPS_BACKUP/base/.inprogress-$run_id"
final="$OPS_BACKUP/base/$run_id"
trap 'rm -f "$OPS_PGPASS"; rm -rf "$work"' EXIT

ops_log "run_id=$run_id start"
pg_basebackup -h "$OPS_SOCKET" -U postgres --no-password -D "$work" -Ft -z -X stream -c fast \
  --manifest-checksums=SHA256 -l "lc-base-$run_id"
pg_verifybackup -n "$work" >/dev/null || ops_die "pg_verifybackup failed for $run_id"
mv "$work" "$final"
trap 'rm -f "$OPS_PGPASS"' EXIT
duration=$(($(date +%s) - start))
ops_log "run_id=$run_id dir=$final bytes=$(du -sb "$final" | cut -f1) duration_s=$duration verify=ok"

# Keep the newest $keep complete bases.
mapfile -t bases < <(find "$OPS_BACKUP/base" -mindepth 1 -maxdepth 1 -type d -name '20*T*Z' -printf '%f\n' | sort)
if ((${#bases[@]} > keep)); then
  for old in "${bases[@]:0:${#bases[@]}-keep}"; do
    rm -rf -- "${OPS_BACKUP:?}/base/$old"
    ops_log "retention removed_base=$old"
  done
  bases=("${bases[@]: -keep}")
fi
oldest=${bases[0]}
segment=$(tar -xzOf "$OPS_BACKUP/base/$oldest/base.tar.gz" backup_label |
  sed -n 's/^START WAL LOCATION: .* (file \([0-9A-F]\{24\}\))$/\1/p')
[[ "$segment" =~ ^[0-9A-F]{24}$ ]] || ops_die "cannot read START WAL segment of base $oldest"
# -x .gz: compressed segments (archive-wal.sh) are matched by their segment name, so one pass prunes <seg>.gz and
# legacy plain <seg> alike; -b also drops the .backup labels of bases older than the oldest kept one. Without -x .gz the
# compressed archive would never be pruned (unbounded growth, I23). .history files are never removed.
pg_archivecleanup -b -x .gz "$OPS_BACKUP/wal" "$segment"
ops_log "wal_pruned_before=$segment oldest_base=$oldest kept_bases=${#bases[@]} result=ok"
printf '%s\n' "$final"
