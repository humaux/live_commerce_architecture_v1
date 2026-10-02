#!/usr/bin/env bash
# File: scripts/agents/ext-agent.sh (was kimi-agent.sh)
# Purpose: run ONE delegated sub-agent task on a third-party model — the owner's Kimi For Coding subscription (model `k3` = K3 by default, or `kimi-for-coding` =
#   K2.8 at 2026-10-01: 1M context, reasoning-only, image/video input) through the Claude Code CLI, which speaks the
#   Anthropic Messages API that Kimi exposes at https://api.kimi.com/coding/. Used by the integrator to offload work
#   that does not need the top tier (docs/delivery/PROCESS.md §3 "Third-party models").
# Usage: [PROVIDER=kimi|deepseek] [MODEL=...] [RESUME=<session_id from a cut-off run's result.json>] bash scripts/agents/ext-agent.sh <worktree> <prompt-file> <out-dir> [effort low|high|max]
#   kimi (subscription, 5-hour quota window): MODEL k3 (default) | kimi-for-coding (K2.8)
#   deepseek (PAY-AS-YOU-GO, owner balance): MODEL deepseek-v4-pro (default) | deepseek-flash; refuses to start below
#   DEEPSEEK_MIN_BALANCE_CNY (default 10, owner 2026-10-02), and a watchdog checks the balance every 60 s during the run and
#   stops the run when it falls below the reserve. Either case writes <repo>/output/ext-agents/DEEPSEEK_LOW_BALANCE (balance,
#   time, task) so the integrator notifies the owner to top up; records the balance before/after in <out>/cost.txt.
# Reads secrets: ~/.config/livecommerce/kimi.env (KIMI_CODE_API_KEY) or deepseek.env (DEEPSEEK_API_KEY), mode 0600, outside the repo. Never printed,
#   never passed on argv (env only), never written under the repo.
# Isolation (the reason this script exists — a third-party model must not inherit the owner's powers):
#   - HOME is a private empty dir (~/.kimi-agent-home): no ~/.claude (no owner CLAUDE.md, no MCP servers such as
#     Gmail/Stripe/Meta), no ~/.ssh (cannot reach the pilot host), no ~/.config (no Stripe/Meta/SMTP secrets).
#   - The environment is rebuilt from scratch (env -i): only PATH, toolchain caches and the Kimi endpoint variables.
#   - --strict-mcp-config with an empty server list; Bash is limited to the allowlist below; reads of the owner's
#     secret locations are denied; edits are allowed only by the CLI's own cwd rules (the worktree).
#   - The worktree must be a dedicated git worktree under .worktrees/ (refused otherwise).
# Never: production hosts, deploys, secrets, buyer PII, merges into release branches (integrator only).
# Status: MODEL_ONLY until calibrated on real units (see docs/delivery/PROCESS.md §3).
{ # bash reads a script while running it: the braces make it parse the whole file first, so editing this file mid-run cannot corrupt a run
set -euo pipefail
wt=${1:?worktree}; prompt=${2:?prompt file}; out=${3:?out dir}; effort=${4:-high}
provider=${PROVIDER:-kimi}
case "$provider" in
  kimi) model=${MODEL:-${KIMI_MODEL:-k3}}; base_url="https://api.kimi.com/coding/"; key_file="$HOME/.config/livecommerce/kimi.env"; key_var=KIMI_CODE_API_KEY
    case "$model" in k3|kimi-for-coding) ;; *) echo "kimi MODEL must be k3 or kimi-for-coding" >&2; exit 2 ;; esac ;;
  deepseek) model=${MODEL:-deepseek-v4-pro}; base_url="https://api.deepseek.com/anthropic"; key_file="$HOME/.config/livecommerce/deepseek.env"; key_var=DEEPSEEK_API_KEY
    case "$model" in deepseek-v4-pro|deepseek-flash) ;; *) echo "deepseek MODEL must be deepseek-v4-pro or deepseek-flash" >&2; exit 2 ;; esac ;;
  *) echo "PROVIDER must be kimi or deepseek" >&2; exit 2 ;;
esac
case "$effort" in low) think=4000 ;; high) think=16000 ;; max) think=32000 ;; *) echo "effort must be low|high|max" >&2; exit 2 ;; esac
wt=$(cd "$wt" && pwd); [[ "$wt" == */.worktrees/* ]] || { echo "refused: $wt is not under .worktrees/" >&2; exit 2; }
[[ -r "$prompt" ]] || { echo "no prompt file" >&2; exit 2; }
mkdir -p "$out"; out=$(cd "$out" && pwd); prompt=$(cd "$(dirname "$prompt")" && pwd)/$(basename "$prompt")
[[ $(stat -f %Lp "$key_file" 2>/dev/null || stat -c %a "$key_file") == 600 ]] || { echo "refused: $key_file must be mode 600" >&2; exit 2; }
# shellcheck disable=SC1090
key=$(. "$key_file"; printf %s "${!key_var}")
sandbox_home="$HOME/.kimi-agent-home"; mkdir -p "$sandbox_home/.claude"; chmod 700 "$sandbox_home"
settings="$out/kimi-settings.json"
cat >"$settings" <<JSON
{
  "permissions": {
    "allow": ["Read", "Edit", "Write", "Glob", "Grep",
      "Bash(git status:*)", "Bash(git diff:*)", "Bash(git log:*)", "Bash(git add:*)", "Bash(git commit:*)", "Bash(git show:*)",
      "Bash(go build:*)", "Bash(go vet:*)", "Bash(go test:*)", "Bash(gofmt:*)",
      "Bash(bash scripts/dev/test-focused.sh:*)", "Bash(bash scripts/dev/test-local.sh:*)", "Bash(bash scripts/dev/test-node.sh:*)",
      "Bash(bash scripts/dev/check-gates.sh:*)", "Bash(bash scripts/dev/check-pkgdocs.sh:*)", "Bash(bash scripts/dev/depmap.sh:*)",
      "Bash(python3 scripts/check_packet.py:*)", "Bash(pnpm -s typecheck:*)", "Bash(pnpm run build:*)", "Bash(node --test:*)",
      "Bash(ls:*)", "Bash(wc:*)", "Bash(grep:*)", "Bash(sed -n:*)", "Bash(head:*)", "Bash(tail:*)"],
    "deny": ["Read(/Users/luolimo/.ssh/**)", "Read(/Users/luolimo/.config/**)", "Read(/Users/luolimo/Downloads/**)",
      "Read(/Users/luolimo/Desktop/**)", "Read(/Users/luolimo/.claude/**)", "Read(/etc/**)",
      "Bash(ssh:*)", "Bash(scp:*)", "Bash(curl:*)", "Bash(wget:*)", "Bash(git push:*)", "Bash(git merge:*)"]
  }
}
JSON
# main checkout root (the shared output/ evidence area), resolved from the git common dir so worktrees agree
repo_root=$(cd "$(git -C "$(dirname "$0")" rev-parse --git-common-dir)/.." && pwd)
balance() { curl -s --max-time 20 -H "Authorization: Bearer $key" https://api.deepseek.com/user/balance | python3 -c 'import json,sys;print(json.load(sys.stdin)["balance_infos"][0]["total_balance"])'; }
if [[ $provider == deepseek ]]; then
  before=$(balance); echo "deepseek balance before: $before CNY" >"$out/cost.txt"
  reserve=${DEEPSEEK_MIN_BALANCE_CNY:-10}
  low_marker="$repo_root/output/ext-agents/DEEPSEEK_LOW_BALANCE"
  below() { python3 -c "import sys; sys.exit(0 if float('$1') < float('$reserve') else 1)"; }
  if below "$before"; then
    printf 'balance=%s reserve=%s time=%s task=%s state=refused\n' "$before" "$reserve" "$(date '+%F %T')" "$out" >>"$low_marker"
    echo "refused: DeepSeek balance $before CNY below reserve $reserve" >&2; exit 2
  fi
fi
cd "$wt"
set +e  # the model run may fail (quota, budget); the cost accounting below must still run
env -i PATH="/Users/luolimo/.local/share/fnm/node-versions/v24.15.0/installation/bin:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin:$HOME/.local/bin:$HOME/go/bin:/usr/local/go/bin" \
  HOME="$sandbox_home" TMPDIR="${TMPDIR:-/tmp}" LANG=en_US.UTF-8 TERM=dumb \
  GOMODCACHE="$(go env GOMODCACHE)" GOCACHE="$(go env GOCACHE)" GOTOOLCHAIN=go1.27.1 \
  npm_config_store_dir="$(pnpm store path 2>/dev/null || true)" PLAYWRIGHT_BROWSERS_PATH="$HOME/Library/Caches/ms-playwright" \
  DOCKER_CONFIG="$HOME/.docker" LC_TEST_LOCK_DIR="${TMPDIR:-/tmp}/lc-test-pg.lock" \
  ANTHROPIC_BASE_URL="$base_url" ANTHROPIC_AUTH_TOKEN="$key" \
  ANTHROPIC_MODEL="$model" ANTHROPIC_SMALL_FAST_MODEL="$model" \
  CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 DISABLE_AUTOUPDATER=1 MAX_THINKING_TOKENS="$think" \
  claude -p "$(cat "$prompt")" ${RESUME:+--resume "$RESUME"} --settings "$settings" --strict-mcp-config --mcp-config '{"mcpServers":{}}' \
    --permission-mode acceptEdits --output-format json >"$out/result.json" 2>"$out/stderr.log" &
run_pid=$!
if [[ $provider == deepseek ]]; then
  # Watchdog: stop the run (not the machine) once the owner's balance drops below the reserve; the marker tells the integrator.
  while kill -0 "$run_pid" 2>/dev/null; do
    sleep 60
    kill -0 "$run_pid" 2>/dev/null || break
    now=$(balance 2>/dev/null) || continue
    if [[ -n $now ]] && below "$now"; then
      printf 'balance=%s reserve=%s time=%s task=%s state=stopped\n' "$now" "$reserve" "$(date '+%F %T')" "$out" >>"$low_marker"
      pkill -TERM -P "$run_pid" 2>/dev/null; kill -TERM "$run_pid" 2>/dev/null
      echo "stopped: DeepSeek balance $now CNY below reserve $reserve" >&2
      break
    fi
  done
fi
wait "$run_pid"
status=$?
set -e
[[ $provider == deepseek ]] && { after=$(balance); echo "deepseek balance after: $after CNY (run cost ≈ $(python3 -c "print(round(float('$before')-float('$after'),2))") CNY)" >>"$out/cost.txt"; cat "$out/cost.txt"; }
python3 - "$out/result.json" <<'PY' || true
import json,sys
d=json.load(open(sys.argv[1])); print("kimi-agent:", "error" if d.get("is_error") else "ok", "turns=%s cost_usd=%s" % (d.get("num_turns"), d.get("total_cost_usd")))
print((d.get("result") or "")[-1500:])
PY
exit $status
}
