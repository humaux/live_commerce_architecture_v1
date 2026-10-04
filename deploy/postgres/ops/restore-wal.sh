#!/usr/bin/env bash
# File: deploy/postgres/ops/restore-wal.sh
# Purpose: PostgreSQL restore_command for the PITR scratch cluster: fetch one archived WAL file from
#   /backup/wal. Reads the compressed form (<file>.gz, written by postgres/archive-wal.sh) when
#   present and falls back to the legacy plain file (segments archived before compression existed,
#   plus .history/.backup/.partial files, which are always plain). Exit 1 = not in the archive
#   (a normal end of replay for PostgreSQL), also for an unreadable .gz.
# Usage: restore-wal.sh %f %p      (restore-pitr.sh: restore_command='bash /ops/restore-wal.sh %f %p')
# Runs as/in: pg-ops container (UID 999, read-only rootfs), /ops is a read-only bind mount.
# Reads env: LC_WAL_ARCHIVE_DIR (default /backup/wal; set only by tests/deploy/ops-disk-guard.sh).
# Reads secrets: none.
# Used by: restore-pitr.sh (the only PITR path); an operator doing a manual restore uses the same command.
# Depends on: bash, gzip, cp.
# Status: DESIGN; verified by tests/deploy/ops-disk-guard.sh OD2 (REAL PG: base + compressed + legacy plain WAL).
# Change rules: never write to the archive; keep both forms readable as long as plain segments can exist.
set -Eeuo pipefail

file=${1:?usage: restore-wal.sh %f %p}
dest=${2:?usage: restore-wal.sh %f %p}
dir=${LC_WAL_ARCHIVE_DIR:-/backup/wal}
[[ "$file" =~ ^[0-9A-Za-z._-]+$ ]] || exit 2

if [[ -f "$dir/$file.gz" ]]; then
  gzip -dc -- "$dir/$file.gz" >"$dest"
elif [[ -f "$dir/$file" ]]; then
  cp -- "$dir/$file" "$dest"
else
  exit 1
fi
