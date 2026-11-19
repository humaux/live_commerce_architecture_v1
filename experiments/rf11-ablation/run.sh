#!/usr/bin/env bash
set -euo pipefail
[[ "$(uname -s)" == Linux && "$GITHUB_REF" == refs/heads/codex/g15-rf11-721-ablation-1119 ]]
[[ -z "${STRIPE_SECRET_KEY:-}${STRIPE_ACCOUNT_ID:-}${STRIPE_BROWSER:-}${STRIPE_SANDBOX:-}${DATABASE_URL:-}" ]]
export GOTOOLCHAIN=go1.27.4 GOFLAGS=-mod=readonly NEXT_TELEMETRY_DISABLED=1
export LC_PLATFORM_HOST=platform.example.invalid LC_ADMIN_HOST=admin.example.invalid LC_COMPANY_CONTACT_EMAIL=contact@example.invalid
export LC_BROWSER_REFUND_FULFILMENT_ACCEPTANCE=1 LC_TEST_LOCK_WAIT=90
out="$PWD/output/g15-rf11"
work="$RUNNER_TEMP/g15-rf11"
browser_root="$PWD/output/playwright/g15-rf11"
mkdir -p "$out" "$work" "$browser_root"
python3 experiments/rf11-ablation/prepare.py "$out"
{
  printf 'runner_os=%s\nrunner_arch=%s\nimage_os=%s\nimage_version=%s\n' "$RUNNER_OS" "$RUNNER_ARCH" "$ImageOS" "$ImageVersion"
  uname -srv
  lscpu | awk -F: '$1 == "Model name" || $1 == "CPU(s)"'
  cat /sys/fs/cgroup/cpu.max /sys/fs/cgroup/memory.max 2>/dev/null || true
  go version
  node --version
  pnpm --version
  pnpm exec playwright --version
  docker version --format '{{.Server.Version}}'
  docker image inspect --format '{{.Id}}' postgres@sha256:4ef4dbc939d61acea57712655ddb4b4ab27419c913f94cca0cd57cb3ea3c2280
} > "$out/runner.txt"
printf 'build-storefront\n' > "$out/stage.txt"
COMMERCE_BUYER_WEB_ENABLED=0 pnpm run build:storefront
for arm in on off; do
  printf 'build-admin-%s\n' "$arm" > "$out/stage.txt"
  cp "$out/component-$arm.tsx" apps/admin/components/OrderRefunds.tsx
  COMMERCE_IDENTITY_ENABLED=0 COMMERCE_FIXTURE_ENABLED=0 pnpm run build:admin
  node scripts/dev/check-gallery-bundle.mjs --expect=absent
  mv apps/admin/.next "$work/admin-$arm"
done
git show HEAD:apps/admin/components/OrderRefunds.tsx > apps/admin/components/OrderRefunds.tsx
printf 'run-samples\n' > "$out/stage.txt"
index=0
while read -r profile arm; do
  index=$((index+1))
  printf 'Running sample %s/6 %s %s\n' "$index" "$profile" "$arm"
  mv "$work/admin-$arm" apps/admin/.next
  sample="$browser_root/$index"
  mkdir -p "$sample"
  log="$work/go-$index.jsonl"
  set +e
  (
    set -euo pipefail
    export G15_RF11_PROFILE="$profile" LC_BROWSER_EVIDENCE_ROOT="$sample"
    source scripts/dev/test-local-runtime.sh
    go test -mod=readonly -race -tags browser -count=1 -timeout=420s -run '^TestBrowserRefund$/^MOCK:' -json ./tests/foundation
  ) > "$log" 2>&1
  code=$?
  set -e
  mv apps/admin/.next "$work/admin-$arm"
  python3 experiments/rf11-ablation/result.py "$out" --sample "$index" --profile "$profile" --arm "$arm" --browser "$sample" --go "$log" --exit-code "$code"
  if [[ "$code" != 0 ]]; then
    printf '::error::RF11 native sample %s (%s/%s) failed with exit %s; see scalar artifact.\n' "$index" "$profile" "$arm" "$code"
  fi
done < <(python3 -c 'import json,sys; print("\n".join(" ".join(pair) for pair in json.load(open(sys.argv[1]))["schedule"]))' "$out/manifest.json")
printf 'adjudicate\n' > "$out/stage.txt"
python3 experiments/rf11-ablation/result.py "$out"
printf 'complete\n' > "$out/stage.txt"
