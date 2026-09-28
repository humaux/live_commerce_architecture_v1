#!/usr/bin/env bash
# File: deploy/postgres/ops/lib.sh
# Purpose: shared helpers for the backup/restore scripts that run INSIDE the pg-ops container:
#   value-free logging, a superuser libpq setup via a tmpfs pgpass file, and fixed paths.
# Runs as/in: service "pg-ops" (postgres image, UID 999, network_mode none, read-only rootfs,
#   /tmp tmpfs), invoked through deploy/scripts/pg-ops.sh. Sourced, never executed.
# Reads env: none required. Paths are fixed by deploy/compose.yml mounts:
#   /var/run/postgresql (pgsocket), /backup (= ${LC_BACKUP_DIR}: dumps/ base/ wal/),
#   /var/lib/postgresql (pitr-scratch volume, NOT pgdata), /ops (these scripts, read-only).
# Reads secrets: /run/secrets/pg_superuser_password -> written once to a 0600 pgpass file on
#   the /tmp tmpfs so no password sits in the environment or argv of pg_dump/psql.
# Used by: backup.sh, basebackup.sh, restore-dump.sh, restore-pitr.sh.
# Depends on: PostgreSQL 18 client tools in the same image as the server (versions match).
# Status: DESIGN; exercised by smoke S28-S31.
# Change rules: keep output free of row values and secrets; only names, counts, sizes, hashes.
set -Eeuo pipefail
umask 077

# Constants used by the scripts that source this file.
# shellcheck disable=SC2034
OPS_DB=live_commerce
# shellcheck disable=SC2034
OPS_SOCKET=/var/run/postgresql
# shellcheck disable=SC2034
OPS_BACKUP=/backup
# shellcheck disable=SC2034
OPS_SCRATCH=/var/lib/postgresql/pitr
OPS_SECRET=/run/secrets/pg_superuser_password

ops_log() { printf '%s pg-ops/%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${OPS_SCRIPT:-${0##*/}}" "$*" >&2; }
ops_die() {
  ops_log "FAIL $1"
  exit "${2:-1}"
}
ops_utc() { date -u +%Y%m%dT%H%M%SZ; }

# ops_auth — libpq connection defaults for the live server over the socket as the superuser.
ops_auth() {
  [[ -r "$OPS_SECRET" ]] || ops_die "missing secret pg_superuser_password"
  OPS_PGPASS=$(mktemp /tmp/pgpass.XXXXXX)
  # pgpass format: host:port:db:user:password; '*' wildcards; the password is 64 hex chars.
  printf '*:*:*:postgres:%s\n' "$(<"$OPS_SECRET")" >"$OPS_PGPASS"
  chmod 0600 "$OPS_PGPASS"
  export PGPASSFILE=$OPS_PGPASS PGHOST=$OPS_SOCKET PGUSER=postgres PGDATABASE=$OPS_DB PGAPPNAME=lc-pg-ops
  trap 'rm -f "$OPS_PGPASS"' EXIT
}

# ops_psql ARGS... — superuser psql, unaligned tuples, stop on error.
ops_psql() { psql -X -q -At -v ON_ERROR_STOP=1 "$@"; }

# ops_sha256 FILE — hex digest only.
ops_sha256() { sha256sum "$1" | cut -d' ' -f1; }
