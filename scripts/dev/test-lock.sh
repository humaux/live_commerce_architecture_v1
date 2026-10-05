# Purpose: the machine-wide PG test lock shared by every PG-holding test runner (one test PG at a time on the dev Mac).
# Depends on: mkdir (atomic on APFS), find -mmin, a background heartbeat subshell; LC_TEST_LOCK_DIR (default $TMPDIR/lc-test-pg.lock).
# Used by: scripts/dev/test-focused.sh, scripts/dev/test-local.sh (source this file, then lc_lock_acquire / lc_lock_release).
# Invariants: liveness is judged ONLY by heartbeat age, never by inspecting the holder's PID — Codex/agent sandboxes cannot
#   always see each other's processes, and the old `kill -0`/`ps` check made them treat live holders as stale and steal the
#   lock (2026-10-06: integrator full suite ran concurrently with three other PG runs). Release removes the lock only if
#   this process still owns it, so a stolen lock is never freed by its previous holder. Every take-over is logged to
#   "$lc_lock_dir.log" for diagnosis.
lc_lock_dir="${LC_TEST_LOCK_DIR:-${TMPDIR:-/tmp}/lc-test-pg.lock}"
lc_lock_hb_pid=""

# lc_lock_acquire waits up to $1 seconds (default 7200) for the lock; returns 2 on timeout. Side effect: starts a
# heartbeat subshell that touches "$lc_lock_dir/heartbeat" every 30 s while this shell lives.
lc_lock_acquire() {
  local deadline=$(( $(date +%s) + ${1:-7200} ))
  until mkdir "$lc_lock_dir" 2>/dev/null; do
    # Stale = heartbeat older than 3 min, or no heartbeat at all on a lock dir older than 3 min (holder died mid-acquire).
    if [[ -n "$(find "$lc_lock_dir/heartbeat" -mmin +3 2>/dev/null)" ]] ||
       { [[ ! -e "$lc_lock_dir/heartbeat" ]] && [[ -n "$(find "$lc_lock_dir" -maxdepth 0 -mmin +3 2>/dev/null)" ]]; }; then
      printf '%s pid %s removed stale lock of pid %s\n' "$(date '+%F %T')" "$$" "$(cat "$lc_lock_dir/pid" 2>/dev/null || echo '?')" >> "$lc_lock_dir.log"
      rm -rf "$lc_lock_dir"
      continue
    fi
    if (( $(date +%s) >= deadline )); then return 2; fi
    sleep 2
  done
  echo $$ > "$lc_lock_dir/pid"
  touch "$lc_lock_dir/heartbeat"
  printf '%s pid %s acquired (%s)\n' "$(date '+%F %T')" "$$" "${0##*/}" >> "$lc_lock_dir.log"
  local owner=$$
  # Detached from stdout/stderr so a caller capturing our output never waits on the heartbeat.
  ( while kill -0 "$owner" 2>/dev/null && [[ "$(cat "$lc_lock_dir/pid" 2>/dev/null)" == "$owner" ]]; do
      touch "$lc_lock_dir/heartbeat" 2>/dev/null; sleep 30
    done ) >/dev/null 2>&1 &
  lc_lock_hb_pid=$!
}

# lc_lock_release stops the heartbeat and removes the lock only if this shell still owns it.
lc_lock_release() {
  if [[ -n "$lc_lock_hb_pid" ]]; then kill "$lc_lock_hb_pid" 2>/dev/null || true; wait "$lc_lock_hb_pid" 2>/dev/null || true; fi
  if [[ "$(cat "$lc_lock_dir/pid" 2>/dev/null)" == "$$" ]]; then rm -rf "$lc_lock_dir"; fi
}
