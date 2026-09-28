#!/usr/bin/env bash
# File: deploy/scripts/pg-ops.sh
# Purpose: host wrapper that runs the database operation scripts inside the one-shot pg-ops
#   container (profile ops; postgres image = same PG 18 client as the server; no network).
# Usage: pg-ops.sh backup [--tag TAG]
#        pg-ops.sh basebackup
#        pg-ops.sh restore-dump <dump dir name | host path under LC_BACKUP_DIR> [restore options]
#        pg-ops.sh restore-pitr --target-time TS [--base NAME] [--drill]
#        pg-ops.sh list
# Runs as/in: deploy host (DB host in the 2-host variant), through lib.sh lc_compose.
# Reads env: compose.env (LC_BACKUP_DIR), LC_DUMP_RETENTION_DAYS, LC_BASE_KEEP,
#   LC_PITR_TIMEOUT_SECONDS, LC_CONFIRM_REPLACE_LIVE, LC_REQUIRE_MEDIA_GATE (forwarded into the
#   container only if set).
# Reads secrets: none on the host; pg-ops mounts pg_superuser_password.
# Used by: cron (deploy/host/crontab.example), deploy.sh upgrade (mandatory backup),
#   smoke.sh S28-S31, docs/runbooks/backup-restore.md.
# Depends on: deploy/postgres/ops/*.sh, compose service pg-ops, running postgres (except PITR).
# Status: DESIGN; verified by smoke S28-S31.
# Change rules: keep restore defaults non-destructive (new DB / scratch cluster).
set -Eeuo pipefail
# shellcheck source-path=SCRIPTDIR source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

lc_load_env "$LC_COMPOSE_ENV"
lc_require_vars LC_BACKUP_DIR
cmd=${1:-}
shift || true

forward=()
for v in LC_DUMP_RETENTION_DAYS LC_BASE_KEEP LC_PITR_TIMEOUT_SECONDS LC_CONFIRM_REPLACE_LIVE LC_REQUIRE_MEDIA_GATE; do
  [[ -n "${!v:-}" ]] && forward+=(-e "$v")
done
# Active profiles + ops (a bare --profile ops would hide the rest of the project from Compose).
run_ops() { lc_compose_with_ops run --rm --no-deps -T "${forward[@]}" pg-ops "$@"; }

case "$cmd" in
backup) run_ops /ops/backup.sh "$@" ;;
basebackup) run_ops /ops/basebackup.sh "$@" ;;
restore-dump)
  src=${1:?usage: pg-ops.sh restore-dump <dump dir> [options]}
  shift
  # Accept a host path under LC_BACKUP_DIR and translate it to the container mount /backup.
  if [[ "$src" == "$LC_BACKUP_DIR"/* ]]; then src="/backup/${src#"$LC_BACKUP_DIR"/}"; fi
  run_ops /ops/restore-dump.sh "$src" "$@"
  ;;
restore-pitr) run_ops /ops/restore-pitr.sh "$@" ;;
list)
  for d in dumps base; do
    echo "== $d ($LC_BACKUP_DIR/$d)"
    if [[ -d "$LC_BACKUP_DIR/$d" ]]; then
      find "$LC_BACKUP_DIR/$d" -mindepth 1 -maxdepth 1 -type d -name '20*' -printf '%f\n' | sort |
        while IFS= read -r n; do printf '%-40s %s\n' "$n" "$(du -sh "$LC_BACKUP_DIR/$d/$n" | cut -f1)"; done
    fi
  done
  echo "== wal segments: $(find "$LC_BACKUP_DIR/wal" -maxdepth 1 -type f 2>/dev/null | wc -l)"
  ;;
*) lc_die "usage: pg-ops.sh backup [--tag T] | basebackup | restore-dump <dir> [...] | restore-pitr --target-time TS [--drill] | list" 2 ;;
esac
