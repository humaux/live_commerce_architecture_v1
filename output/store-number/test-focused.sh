#!/usr/bin/env bash
# Safe wrapper for this unit: baseline runners do not all share bounded locking.
# Filename deliberately matches the focused runner's live-holder PID check.
set -u
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/store-number || exit 2
export LC_TEST_LOCK_WAIT=7200
label="${1:?evidence label required}"
[[ "$label" =~ ^[a-z0-9-]+$ ]] || exit 2
shift
task_lock="${TMPDIR:-/tmp}/lc-test-pg.lock"
task_deadline=$(( $(date +%s) + LC_TEST_LOCK_WAIT ))
until mkdir "$task_lock" 2>/dev/null; do
  if (( $(date +%s) >= task_deadline )); then
    echo 'Timed out waiting for shared PG lock; no foreign lock changed.'
    exit 124
  fi
  sleep 2
done
printf '%s\n' "$$" > "$task_lock/pid"
cleanup() {
  if [[ "$(cat "$task_lock/pid" 2>/dev/null)" == "$$" ]]; then
    rm "$task_lock/pid"
    rmdir "$task_lock"
  fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM
export LC_TEST_LOCK_DIR="$PWD/output/store-number/child-pg.lock"
"$@" > "output/store-number/$label.log" 2>&1
rc=$?
printf '%s\t%s\n' "$label" "$rc" | tee -a output/store-number/exits.tsv
exit "$rc"
