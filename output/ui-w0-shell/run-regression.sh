#!/usr/bin/env bash
# Requested G-UI7 modes, serial: preserve every original failure and continue other items.
set -u
cd "$(dirname "$0")/../.."
out=output/ui-w0-shell
printf 'mode\texit\tstarted_utc\tfinished_utc\n' > "$out/regression-exits.tsv"
for mode in browser-admin-legacy browser-catalog-core browser-catalog-media browser-promotions browser-ops-polish browser-customers-billing browser-studio-ui browser-merchant-orders-ui browser-cvs browser-meta-connect browser-design; do
  started=$(date -u +%FT%TZ)
  bash scripts/dev/test-local.sh "--$mode" > "$out/$mode.log" 2>&1
  rc=$?
  finished=$(date -u +%FT%TZ)
  printf '%s\t%s\t%s\t%s\n' "$mode" "$rc" "$started" "$finished" >> "$out/regression-exits.tsv"
  printf '%s exit=%s\n' "$mode" "$rc"
done
