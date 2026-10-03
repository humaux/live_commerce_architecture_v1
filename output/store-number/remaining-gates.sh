#!/usr/bin/env bash
set -u
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/store-number || exit 2
export LC_TEST_LOCK_WAIT=7200
export LC_BROWSER_EVIDENCE_ROOT="$PWD/output/store-number/browser-evidence"
for mode in password-auth admin-shell click-sweep; do
  bash output/store-number/test-focused.sh "browser-$mode" bash scripts/dev/test-local.sh "--browser-$mode"
done
export LC_RELEASE_GATE_OUT="$PWD/output/store-number/g07"
bash output/store-number/test-focused.sh full-g07 bash scripts/dev/release-gate.sh --strict --only G07
