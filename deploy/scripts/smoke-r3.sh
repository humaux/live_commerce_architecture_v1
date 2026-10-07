#!/usr/bin/env bash
# Purpose: unauthenticated R3 route probes shared by static control tests and full deploy smoke.
# Depends on: lc_r3_request(method,path,body,idempotency_key) supplied by smoke.sh; frozen admin routes.
# Used by: smoke.sh S48/S49 and tests/deploy/deploy-prep-r3.test.mjs. No credentials, writes or provider calls.

# lc_r3_smoke CLAIMS_ENABLED ADS_ENABLED — every mounted route must deny with 401.
# 404 is accepted only when that route's feature is explicitly off in this isolated fixture.
lc_r3_smoke() {
  local claims=$1 ads=$2 name method route body key mount expected actual bad=0
  [[ "$claims" =~ ^[01]$ && "$ads" =~ ^[01]$ ]] || return 1
  while IFS='|' read -r name method route body key mount; do
    expected=401
    if [[ "$mount" == claims && "$claims" == 0 || "$mount" == ads && "$ads" == 0 ]]; then expected=404; fi
    actual=$(lc_r3_request "$method" "$route" "$body" "$key") || actual=ERROR
    printf 'R3 %s status=%s expected=%s\n' "$name" "$actual" "$expected"
    [[ "$actual" == "$expected" ]] || bad=1
  done <<'PROBES'
operations|GET|/v1/admin/stores/00000000-0000-4000-8000-000000000001/operations|||always
returns|GET|/v1/admin/stores/00000000-0000-4000-8000-000000000001/returns|||always
settlements|GET|/v1/admin/stores/00000000-0000-4000-8000-000000000001/settlements|||always
keyword-simulate|POST|/v1/admin/stores/00000000-0000-4000-8000-000000000001/live-sessions/00000000-0000-4000-8000-000000000002/claims/simulate|{"comment":"A1"}||claims
ads-unbind|POST|/v1/admin/stores/00000000-0000-4000-8000-000000000001/ads/meta/unbind|{"ad_account_id":"9001"}|smoke-r3-unbind|ads
catalog-feed|GET|/v1/admin/stores/00000000-0000-4000-8000-000000000001/ads/catalog-feed|||ads
PROBES
  return "$bad"
}
