#!/usr/bin/env bash
# Unit-local queue wrapper: never removes a foreign lock. Child gets its own lock.
set -u
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-tw-regulation || exit 2
export LC_TEST_LOCK_WAIT=14400
export LC_RELEASE_GATE_OUT="$PWD/output/ads-tw-regulation/release-gate"
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
export LC_TEST_LOCK_DIR="$PWD/output/ads-tw-regulation/child-pg.lock"
"$@" > "output/ads-tw-regulation/$label.log" 2>&1
rc=$?
printf '%s\t%s\n' "$label" "$rc" | tee -a output/ads-tw-regulation/exits.tsv
exit "$rc"
