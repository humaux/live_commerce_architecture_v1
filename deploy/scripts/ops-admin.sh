#!/usr/bin/env bash
# File: deploy/scripts/ops-admin.sh
# Purpose: the ONLY sanctioned way to run the operator CLIs (stripe-admin, meta-admin) against a
#   deployed stack: a one-shot container of profile `ops` that holds the registrar logins and the
#   operator's own inputs, never one of the app containers (api/admin/storefront/workers hold neither).
#   Prints the CLI's JSON result line (IDs and versions only); never prints a secret.
# Usage: ops-admin.sh stripe-admin register|rotate|webhook|qualify|method [flags]   (stripe-psp-v1 §13)
#        ops-admin.sh meta-admin page-token [flags]                                  (meta-claims-intake-v1 §7)
#   Operator inputs are read from the caller's environment or, on a terminal, prompted WITHOUT echo:
#     stripe-admin register|rotate : STRIPE_SECRET_KEY (sk_test_/rk_test_ only), STRIPE_ACCOUNT_ID
#     stripe-admin webhook         : STRIPE_WEBHOOK_SECRET [, STRIPE_WEBHOOK_SECRET_NEXT]  (whsec_...)
#     stripe-admin qualify SANDBOX : STRIPE_SECRET_KEY, STRIPE_ACCOUNT_ID, STRIPE_SANDBOX=1
#     meta-admin page-token        : META_PAGE_ACCESS_TOKEN
# Runs as/in: deploy host (root or a docker-group user), through lib.sh lc_compose_with_ops.
# Reads env: compose.env, and the input variables above (forwarded into the one-shot container by
#   NAME only, so the value is never in argv, `ps` or a file; it exists in that container's config
#   until `--rm` removes it, readable only by host root, exactly like the secret files).
# Reads secrets: none directly; the container mounts its own DSN + keyrings (compose.yml).
# Used by: docs/runbooks/deploy.md §Stripe / §Meta, docs/runbooks/merchant-onboarding.md; smoke S44.
# Depends on: compose services stripe-admin / meta-admin, a running postgres with migrations and
#   provisioned logins (deploy.sh first).
# Status: DESIGN; the registrar-login and container wiring is verified by smoke S13 and S44, a real
#   SANDBOX registration is owner-run (NOT_RUN in CI: needs the owner's Stripe test key).
# Change rules: keep the allowlists below equal to the CLIs' subcommands; never accept a live key or
#   `--profile LIVE` here (the CLIs refuse LIVE in code too; this is the earlier, clearer failure);
#   never add `set -x`; never echo a forwarded variable.
set -Eeuo pipefail
# shellcheck source-path=SCRIPTDIR source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  lc_die "usage: ops-admin.sh stripe-admin register|rotate|webhook|qualify|method [flags] | meta-admin page-token [flags]" 2
}
tool=${1:-}
sub=${2:-}
[[ -n "$tool" && -n "$sub" ]] || usage
shift 2
args=("$@")
case "$tool:$sub" in
stripe-admin:register | stripe-admin:rotate | stripe-admin:webhook | stripe-admin:qualify | stripe-admin:method) ;;
meta-admin:page-token) ;;
*) usage ;;
esac

# The CLIs refuse LIVE themselves; refuse it here first so the operator sees why.
for ((i = 0; i < ${#args[@]}; i++)); do
  case "${args[$i]}" in
  --profile=LIVE | -profile=LIVE) lc_die "LIVE is refused: no live activation without owner approval (stripe-psp-v1 §5.2)" ;;
  --profile | -profile) [[ "${args[$((i + 1))]:-}" == LIVE ]] && lc_die "LIVE is refused: no live activation without owner approval (stripe-psp-v1 §5.2)" ;;
  esac
done

lc_load_env "$LC_COMPOSE_ENV"

# need NAME secret|plain REGEX HINT — require the variable, prompting (no echo) when on a terminal.
need() {
  local name=$1 kind=$2 re=$3 hint=$4 v
  if [[ -z "${!name:-}" ]]; then
    [[ -t 0 ]] || lc_die "$name is required ($hint); export it or run from a terminal to be prompted"
    if [[ "$kind" == secret ]]; then
      read -r -s -p "$name ($hint): " v
      echo >&2
    else
      read -r -p "$name ($hint): " v
    fi
    export "$name=$v"
    unset v
  fi
  [[ "${!name}" =~ $re ]] || lc_die "$name has an unexpected format ($hint)"
  forward+=(-e "$name")
}

forward=()
case "$tool:$sub" in
stripe-admin:register | stripe-admin:rotate)
  need STRIPE_SECRET_KEY secret '^(sk|rk)_test_[A-Za-z0-9_]{8,}$' "Stripe TEST key sk_test_/rk_test_; live keys are refused"
  need STRIPE_ACCOUNT_ID plain '^acct_[A-Za-z0-9]{6,64}$' "acct_..."
  ;;
stripe-admin:webhook)
  need STRIPE_WEBHOOK_SECRET secret '^whsec_[A-Za-z0-9_+/=-]{8,}$' "signing secret whsec_... of the endpoint"
  if [[ -n "${STRIPE_WEBHOOK_SECRET_NEXT:-}" ]]; then
    need STRIPE_WEBHOOK_SECRET_NEXT secret '^whsec_[A-Za-z0-9_+/=-]{8,}$' "next signing secret whsec_..."
  fi
  ;;
stripe-admin:qualify)
  # Only SANDBOX qualify reaches Stripe; the CLI decides from --profile, the inputs are forwarded when set.
  if [[ -n "${STRIPE_SECRET_KEY:-}" ]]; then
    need STRIPE_SECRET_KEY secret '^(sk|rk)_test_[A-Za-z0-9_]{8,}$' "Stripe TEST key"
    need STRIPE_ACCOUNT_ID plain '^acct_[A-Za-z0-9]{6,64}$' "acct_..."
    [[ "${STRIPE_SANDBOX:-}" == 1 ]] || lc_die "STRIPE_SANDBOX=1 is required (qualify creates and expires a real sandbox Checkout Session)"
    forward+=(-e STRIPE_SANDBOX)
  fi
  ;;
meta-admin:page-token)
  need META_PAGE_ACCESS_TOKEN secret '^[^[:space:]]{16,4096}$' "Page access token; scopes are attested by --scopes"
  ;;
esac

# `run` REPLACES the service command, so the binary is named again. --no-deps: never start
# postgres/migrate from here (deploy.sh owns ordering). -T: no TTY, the prompt above already ran.
lc_info "ops-admin $tool $sub (one-shot container, profile ops)"
rc=0
lc_compose_with_ops run --rm --no-deps -T "${forward[@]}" "$tool" "/app/bin/$tool" "$sub" "${args[@]}" || rc=$?
unset STRIPE_SECRET_KEY STRIPE_WEBHOOK_SECRET STRIPE_WEBHOOK_SECRET_NEXT META_PAGE_ACCESS_TOKEN
# Audit trail: who ran which subcommand and how it ended. No flag values (they may name tenants).
if [[ -n "${LC_STATE_DIR:-}" && -d "${LC_STATE_DIR:-}" && -w "${LC_STATE_DIR:-}" ]]; then
  printf '%s ops-admin operator=%s tool=%s sub=%s exit=%s\n' "$(lc_ts)" "${SUDO_USER:-${USER:-unknown}}" "$tool" "$sub" "$rc" \
    >>"$LC_STATE_DIR/ops-admin.log" || true
fi
exit "$rc"
