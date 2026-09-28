#!/usr/bin/env bash
# File: deploy/scripts/build-images.sh
# Purpose: build the four deploy images, tagged with the 12-char commit (+ "-dirty" when the
#   checkout has local changes) and OCI revision labels: lc-go, lc-admin, lc-storefront, lc-caddy.
# Usage: build-images.sh [--tag TAG] [--only go|admin|storefront|caddy]... [--evidence DIR]
# Runs as/in: build host with Docker >= 25 (BuildKit cache mounts). Context = repo root
#   (.dockerignore filters it); caddy uses deploy/docker as context.
# Reads env: LC_IMAGE_PREFIX (default lc); HTTP_PROXY/HTTPS_PROXY/NO_PROXY are passed as build
#   args only when set (Docker predefined args; not persisted in image config).
# Reads secrets: none — images never contain secrets or env files.
# Used by: operators before deploy.sh (IMAGE_TAG in compose.env = printed tag), smoke.sh S07.
# Depends on: deploy/docker/*.Dockerfile, git (commit id), docker.
# Exit: 0 built, 1 build failed, 3 BLOCKED (cmd/migrate missing = REQUIRES_INTEGRATOR I1;
#   the Go image would be undeployable without it, so nothing is faked).
# Status: DESIGN; S07 is BLOCKED until cmd/migrate lands.
# Change rules: keep tags immutable (never reuse a tag for different content; "-dirty" tags are
#   for rehearsal only and must not be deployed to production).
set -Eeuo pipefail
# shellcheck source-path=SCRIPTDIR source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

tag="" only=() evidence=""
while (($#)); do
  case "$1" in
  --tag)
    tag=${2:?}
    shift 2
    ;;
  --only)
    only+=("${2:?}")
    shift 2
    ;;
  --evidence)
    evidence=${2:?}
    shift 2
    ;;
  *) lc_die "usage: build-images.sh [--tag TAG] [--only go|admin|storefront|caddy]... [--evidence DIR]" 2 ;;
  esac
done
((${#only[@]})) || only=(go admin storefront caddy)
tag=${tag:-$(lc_git_tag)}
[[ "$tag" =~ ^[A-Za-z0-9._-]{1,64}$ ]] || lc_die "invalid tag"
sha=$(git -C "$LC_REPO_ROOT" rev-parse HEAD)
prefix=${LC_IMAGE_PREFIX:-lc}

for img in "${only[@]}"; do
  if [[ "$img" == go && ! -d "$LC_REPO_ROOT/cmd/migrate" ]]; then
    lc_error "BLOCKED: cmd/migrate is missing (REQUIRES_INTEGRATOR I1, deploy-design §7); lc-go would be undeployable"
    exit 3
  fi
done

proxy_args=()
for v in HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy; do
  [[ -n "${!v:-}" ]] && proxy_args+=(--build-arg "$v")
done

build() { # name dockerfile context
  lc_info "building $prefix-$1:$tag"
  docker build --pull=false -f "$2" -t "$prefix-$1:$tag" --build-arg "GIT_SHA=$sha" "${proxy_args[@]}" "$3"
}
for img in "${only[@]}"; do
  case "$img" in
  go) build go "$LC_DEPLOY_DIR/docker/go.Dockerfile" "$LC_REPO_ROOT" ;;
  admin) build admin "$LC_DEPLOY_DIR/docker/admin.Dockerfile" "$LC_REPO_ROOT" ;;
  storefront) build storefront "$LC_DEPLOY_DIR/docker/storefront.Dockerfile" "$LC_REPO_ROOT" ;;
  caddy) build caddy "$LC_DEPLOY_DIR/docker/caddy.Dockerfile" "$LC_DEPLOY_DIR/docker" ;;
  *) lc_die "unknown image $img" 2 ;;
  esac
done

report=$(for img in "${only[@]}"; do
  docker image inspect "$prefix-$img:$tag" --format \
    '{"image":"{{index .RepoTags 0}}","id":"{{.Id}}","size":{{.Size}},"user":"{{.Config.User}}","revision":"{{index .Config.Labels "org.opencontainers.image.revision"}}"}'
done)
printf '%s\n' "$report"
if [[ -n "$evidence" ]]; then
  mkdir -p "$evidence"
  printf '%s\n' "$report" >"$evidence/images.jsonl"
fi
lc_info "IMAGE_TAG=$tag (set it in compose.env or pass to deploy.sh upgrade)"
