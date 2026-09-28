#!/usr/bin/env bash
# File: deploy/postgres/ops/restore-pitr.sh
# Purpose: point-in-time recovery into a PRIVATE scratch cluster inside pg-ops (deploy-design
#   §15): untar a base backup into the pitr-scratch volume, replay archived WAL up to
#   --target-time with recovery_target_action=pause, run verify.sql against the paused
#   cluster, stop it. Never touches live PGDATA (pg-ops does not mount pgdata) and never
#   archives into the live WAL archive (archive_mode=off for the scratch server).
# Usage: restore-pitr.sh --target-time '<timestamptz>' [--base <UTC dir name>] [--drill]
#   --drill wipes the scratch cluster afterwards (quarterly drill / smoke S31). Without it the
#   stopped cluster stays in the pitr-scratch volume for the MANUAL promotion procedure
#   (docs/runbooks/backup-restore.md §PITR 提升), which is never automatic.
# Runs as/in: pg-ops container (UID 999, no network) via `deploy/scripts/pg-ops.sh restore-pitr`.
# Reads env: LC_PITR_TIMEOUT_SECONDS (default 1800).
# Reads secrets: pg_superuser_password (pgpass on tmpfs; the restored cluster keeps the
#   superuser password it had at backup time).
# Used by: quarterly PITR drill + manual PITR promotion (docs/runbooks/backup-restore.md §4, §6),
#   smoke.sh S31.
# Depends on: basebackup.sh output in /backup/base, archived WAL in /backup/wal, ops/verify.sql.
# Status: DESIGN; verified by smoke S31 (records the drill duration = RTO sample).
# Change rules: keep listen_addresses='' (socket only) and archive_mode=off for the scratch server.
export OPS_SCRIPT=restore-pitr
# shellcheck source=deploy/postgres/ops/lib.sh
source /ops/lib.sh

target_time="" base="" drill=0
while (($#)); do
  case "$1" in
  --target-time)
    target_time=${2:?--target-time needs a value}
    shift 2
    ;;
  --base)
    base=${2:?--base needs a value}
    shift 2
    ;;
  --drill) drill=1 && shift ;;
  *) ops_die "usage: restore-pitr.sh --target-time TS [--base NAME] [--drill]" 2 ;;
  esac
done
[[ -n "$target_time" ]] || ops_die "--target-time is required" 2
[[ "$target_time" =~ ^[0-9A-Za-z:.+\ _-]{8,40}$ ]] || ops_die "invalid --target-time" 2
timeout=${LC_PITR_TIMEOUT_SECONDS:-1800}
[[ "$timeout" =~ ^[1-9][0-9]{0,5}$ ]] || ops_die "invalid LC_PITR_TIMEOUT_SECONDS" 2
if [[ -z "$base" ]]; then
  base=$(find "$OPS_BACKUP/base" -mindepth 1 -maxdepth 1 -type d -name '20*T*Z' -printf '%f\n' 2>/dev/null | sort | tail -n1)
fi
[[ "$base" =~ ^20[0-9]{6}T[0-9]{6}Z$ && -f "$OPS_BACKUP/base/$base/base.tar.gz" ]] || ops_die "no usable base backup (run pg-ops.sh basebackup)"

ops_auth
start=$(date +%s)
data="$OPS_SCRATCH/pgdata"
sock="$OPS_SCRATCH/sock"
rm -rf -- "${OPS_SCRATCH:?}"
mkdir -m 0700 "$OPS_SCRATCH" "$data" "$sock"
cleanup() {
  pg_ctl -D "$data" -m fast stop >/dev/null 2>&1 || true
  rm -f "$OPS_PGPASS"
  if ((drill)); then rm -rf -- "${OPS_SCRATCH:?}"; fi
}
trap cleanup EXIT

tar -xzf "$OPS_BACKUP/base/$base/base.tar.gz" -C "$data"
tar -xzf "$OPS_BACKUP/base/$base/pg_wal.tar.gz" -C "$data/pg_wal"
touch "$data/recovery.signal"
printf 'local all postgres scram-sha-256\n' >"$OPS_SCRATCH/pg_hba.conf"
# Hot standby refuses to replay with any of these below the primary's values ("recovery
# aborted because of insufficient parameter settings"), so take them from the backup's
# pg_control instead of hardcoding (VERIFIED_LOCAL: max_connections=20 aborted recovery).
ctl() { pg_controldata "$data" | sed -n "s/^$1 setting: *//p"; }
mc=$(ctl max_connections) mwp=$(ctl max_worker_processes) mws=$(ctl max_wal_senders)
mpx=$(ctl max_prepared_xacts) mlx=$(ctl max_locks_per_xact)
[[ "$mc$mwp$mws$mpx$mlx" =~ ^[0-9]+$ ]] || ops_die "cannot read primary settings from pg_control"
ops_log "base=$base target_time=$target_time scratch=$data start"

pg_ctl -D "$data" -l "$OPS_SCRATCH/pitr.log" -w -t 600 -o "\
 -c listen_addresses='' -c unix_socket_directories='$sock' -c hba_file='$OPS_SCRATCH/pg_hba.conf' \
 -c archive_mode=off -c shared_buffers=128MB -c logging_collector=off \
 -c max_connections=$mc -c max_worker_processes=$mwp -c max_wal_senders=$mws \
 -c max_prepared_transactions=$mpx -c max_locks_per_transaction=$mlx \
 -c restore_command='cp /backup/wal/%f %p' -c recovery_target_time='$target_time' \
 -c recovery_target_action=pause" start >/dev/null ||
  ops_die "scratch server did not start (see $OPS_SCRATCH/pitr.log inside pg-ops)"

state=""
for ((i = 0; i < timeout; i += 2)); do
  state=$(PGHOST=$sock ops_psql -d postgres -c "SELECT pg_get_wal_replay_pause_state();" 2>/dev/null || true)
  [[ "$state" == paused ]] && break
  if ! pg_ctl -D "$data" status >/dev/null 2>&1; then
    ops_die "recovery ended before the target was reached (target later than the newest archived WAL?)"
  fi
  sleep 2
done
[[ "$state" == paused ]] || ops_die "recovery did not reach the pause point within ${timeout}s"
replay=$(PGHOST=$sock ops_psql -d postgres -c "SELECT pg_last_xact_replay_timestamp();")
ops_log "paused at_target=yes last_replayed_xact=$replay"
[[ "${LC_REQUIRE_MEDIA_GATE:-0}" =~ ^[01]$ ]] || ops_die "LC_REQUIRE_MEDIA_GATE must be 0 or 1" 2
PGHOST=$sock ops_psql -d "$OPS_DB" -v require_media="${LC_REQUIRE_MEDIA_GATE:-0}" -f /ops/verify.sql >&2 ||
  ops_die "verify.sql failed on the PITR cluster"
pg_ctl -D "$data" -m fast stop >/dev/null
duration=$(($(date +%s) - start))
ops_log "base=$base duration_s=$duration verify=PASS drill=$drill result=ok"
if ((!drill)); then
  ops_log "stopped PITR cluster kept in the pitr-scratch volume (pgdata/); promotion is manual"
fi
printf 'pitr_duration_seconds=%s\n' "$duration"
