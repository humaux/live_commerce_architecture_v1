#!/usr/bin/env bash
# File: scripts/agents/kimi-agent.sh
# Purpose: run ONE delegated sub-agent task on the owner's Kimi For Coding subscription (model `k3` = K3 by default, or `kimi-for-coding` =
#   K2.8 at 2026-10-01: 1M context, reasoning-only, image/video input) through the Claude Code CLI, which speaks the
#   Anthropic Messages API that Kimi exposes at https://api.kimi.com/coding/. Used by the integrator to offload work
#   that does not need the top tier (docs/delivery/PROCESS.md §3 "Third-party models").
# Usage: [KIMI_MODEL=k3|kimi-for-coding] bash scripts/agents/kimi-agent.sh <worktree> <prompt-file> <out-dir> [effort low|high|max]
# Reads secrets: ~/.config/livecommerce/kimi.env (KIMI_CODE_API_KEY, mode 0600, outside the repo). Never printed,
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
set -euo pipefail
wt=${1:?worktree}; prompt=${2:?prompt file}; out=${3:?out dir}; effort=${4:-high}
# KIMI_MODEL: k3 (default; flagship, 1M ctx, reasoning-only — judgment-heavier work) or kimi-for-coding (K2.8, coding/bulk).
model=${KIMI_MODEL:-k3}; case "$model" in k3|kimi-for-coding) ;; *) echo "KIMI_MODEL must be k3 or kimi-for-coding" >&2; exit 2 ;; esac
case "$effort" in low) think=4000 ;; high) think=16000 ;; max) think=32000 ;; *) echo "effort must be low|high|max" >&2; exit 2 ;; esac
wt=$(cd "$wt" && pwd); [[ "$wt" == */.worktrees/* ]] || { echo "refused: $wt is not under .worktrees/" >&2; exit 2; }
[[ -r "$prompt" ]] || { echo "no prompt file" >&2; exit 2; }
mkdir -p "$out"; out=$(cd "$out" && pwd); prompt=$(cd "$(dirname "$prompt")" && pwd)/$(basename "$prompt")
key_file="$HOME/.config/livecommerce/kimi.env"
[[ $(stat -f %Lp "$key_file" 2>/dev/null || stat -c %a "$key_file") == 600 ]] || { echo "refused: $key_file must be mode 600" >&2; exit 2; }
# shellcheck disable=SC1090
key=$(. "$key_file"; printf %s "$KIMI_CODE_API_KEY")
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
cd "$wt"
env -i PATH="/Users/luolimo/.local/share/fnm/node-versions/v24.15.0/installation/bin:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin:$HOME/.local/bin:$HOME/go/bin:/usr/local/go/bin" \
  HOME="$sandbox_home" TMPDIR="${TMPDIR:-/tmp}" LANG=en_US.UTF-8 TERM=dumb \
  GOMODCACHE="$(go env GOMODCACHE)" GOCACHE="$(go env GOCACHE)" GOTOOLCHAIN=local \
  npm_config_store_dir="$(pnpm store path 2>/dev/null || true)" PLAYWRIGHT_BROWSERS_PATH="$HOME/Library/Caches/ms-playwright" \
  DOCKER_CONFIG="$HOME/.docker" LC_TEST_LOCK_DIR="${TMPDIR:-/tmp}/lc-test-pg.lock" \
  ANTHROPIC_BASE_URL="https://api.kimi.com/coding/" ANTHROPIC_AUTH_TOKEN="$key" \
  ANTHROPIC_MODEL="$model" ANTHROPIC_SMALL_FAST_MODEL="$model" \
  CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 DISABLE_AUTOUPDATER=1 MAX_THINKING_TOKENS="$think" \
  claude -p "$(cat "$prompt")" --settings "$settings" --strict-mcp-config --mcp-config '{"mcpServers":{}}' \
    --permission-mode acceptEdits --output-format json >"$out/result.json" 2>"$out/stderr.log"
status=$?
python3 - "$out/result.json" <<'PY' || true
import json,sys
d=json.load(open(sys.argv[1])); print("kimi-agent:", "error" if d.get("is_error") else "ok", "turns=%s cost_usd=%s" % (d.get("num_turns"), d.get("total_cost_usd")))
print((d.get("result") or "")[-1500:])
PY
exit $status
