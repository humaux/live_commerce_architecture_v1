#!/usr/bin/env bash
set -u
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/store-number
run() {
  local label="$1"; shift
  "$@" >"output/store-number/${label}.log" 2>&1
  local code=$?
  printf '%s\t%s\n' "$label" "$code" >>output/store-number/exits.tsv
  printf '%s exit=%s\n' "$label" "$code"
}
run go-build go build ./...
run go-vet go vet ./...
run gofmt bash -c 'gofmt -l $(git ls-files "*.go"); test -z "$(gofmt -l $(git ls-files "*.go"))"'
run go-unit go test ./internal/identityhttp ./internal/storehandles
run check-gates bash scripts/dev/check-gates.sh
run test-node bash scripts/dev/test-node.sh
run admin-tsc pnpm --filter admin exec tsc --noEmit
