#!/usr/bin/env bash
# File: deploy/postgres/archive-wal.sh
# Purpose: PostgreSQL archive_command. WAL segments are archived gzip-compressed as <segment>.gz:
#   archive_timeout=60s forces a switch every minute and a forced-switch segment is mostly zeros,
#   so it compresses from 16 MiB to tens of KiB (uncompressed it was ~23 GB/day on an idle DB and
#   filled the pilot disk, 2026-10-03). .history, .backup and .partial files stay plain copies.
#   Idempotent: an already archived segment succeeds when its (decompressed) content equals the
#   source (a crash between the rename and PostgreSQL's bookkeeping must not wedge the archiver);
#   DIFFERENT content under the same name fails loudly (watchdog W4). A legacy plain copy of the
#   same segment (archived before compression existed) counts as already archived.
#   Atomic: <target>.part -> read back and compare -> fsync -> rename -> fsync the directory.
#   PostgreSQL deletes the local segment once this exits 0, so never exit 0 on an unverified file.
# Usage: archive-wal.sh %p %f      (postgresql.conf: archive_command = 'bash /etc/postgresql/archive-wal.sh %p %f')
# Runs as/in: service "postgres" (UID 999, read-only rootfs), bind-mounted read-only at
#   /etc/postgresql/archive-wal.sh; the archive is the bind mount /backup/wal (0700 999:999).
# Reads env: LC_WAL_ARCHIVE_DIR (default /backup/wal; set only by tests/deploy/ops-disk-guard.sh).
# Reads secrets: none.
# Used by: postgresql.conf archive_command; read back by ops/restore-wal.sh (restore_command) and
#   pruned by ops/basebackup.sh (pg_archivecleanup -b -x .gz).
# Depends on: bash, gzip, cmp, sync (all in the pinned postgres image).
# Status: DESIGN; verified by tests/deploy/ops-disk-guard.sh OD1 (REAL PG), smoke S30/S42.
# Change rules: keep exit 0 only for a verified, durable archive file; keep the 60 s archive_timeout
#   (RPO) and fix volume with compression + retention, never by archiving less often.
set -Eeuo pipefail
umask 077

src=${1:?usage: archive-wal.sh %p %f}
name=${2:?usage: archive-wal.sh %p %f}
dir=${LC_WAL_ARCHIVE_DIR:-/backup/wal}
[[ "$name" =~ ^[0-9A-Za-z._-]+$ && -f "$src" ]] || {
  echo "archive-wal: bad arguments" >&2
  exit 2
}

if [[ "$name" =~ ^[0-9A-F]{24}$ ]]; then
  # A WAL segment. Is it already archived, compressed or as a legacy plain copy?
  if [[ -f "$dir/$name.gz" ]]; then
    gzip -dc -- "$dir/$name.gz" | cmp -s - "$src" && exit 0
    echo "archive-wal: $name.gz already archived with different or unreadable content" >&2
    exit 1
  fi
  if [[ -f "$dir/$name" ]]; then
    cmp -s -- "$src" "$dir/$name" && exit 0
    echo "archive-wal: $name already archived with different content" >&2
    exit 1
  fi
  target="$dir/$name.gz"
else
  # .history / .backup / .partial: small, kept as plain copies.
  if [[ -f "$dir/$name" ]]; then
    cmp -s -- "$src" "$dir/$name" && exit 0
    echo "archive-wal: $name already archived with different content" >&2
    exit 1
  fi
  target="$dir/$name"
fi

tmp="$target.part"
trap 'rm -f -- "$tmp"' EXIT # a full disk must not leave a half-written .part behind
if [[ "$target" == *.gz ]]; then
  gzip -n -c <"$src" >"$tmp"
  gzip -dc -- "$tmp" | cmp -s - "$src" || {
    echo "archive-wal: $name read-back mismatch" >&2
    exit 1
  }
else
  cp -- "$src" "$tmp"
fi
sync -- "$tmp"
mv -- "$tmp" "$target"
sync -- "$dir"
