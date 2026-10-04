#!/usr/bin/env bash
# File: deploy/scripts/prune-docker.sh
# Purpose: bound Docker growth on the build/deploy host (ops-disk-guard D5). Every host build left a
#   BuildKit cache layer and a full set of lc-* images behind (~6.5 GB over 4 builds on the pilot) and
#   nothing ever pruned them; together with the WAL archive that filled the disk (2026-10-03).
#   1. removes ${LC_IMAGE_PREFIX:-lc}-{go,admin,storefront,caddy} images whose tag is NOT in the keep set:
#      the tag just built (--keep), the running compose.env IMAGE_TAG, and the last two distinct tags in
#      deployments.log (= running + PREVIOUS deployed tag, so `deploy.sh app-rollback <previous>` keeps working);
#   2. `docker builder prune -f` down to 3 GB of cache.
#   Never forces: an image a container (even a stopped one) uses cannot be removed and is reported as kept.
#   Does nothing on a host without deployments.log (a developer machine, or a host that never deployed), so it
#   cannot delete the images of other work.
# Usage: prune-docker.sh [--keep TAG]... [--dry-run]
# Runs as/in: build/deploy host (root); called by build-images.sh after a successful build, also by hand.
# Reads env: LC_IMAGE_PREFIX (default lc), LC_STATE_DIR (default /var/lib/live-commerce), LC_COMPOSE_ENV
#   (read as a file: IMAGE_TAG, LC_IMAGE_PREFIX, LC_STATE_DIR).
# Reads secrets: none.
# Used by: build-images.sh, docs/runbooks/incident.md (disk full), tests/deploy/ops-disk-guard.sh OD6.
# Depends on: lib.sh, docker, awk. Plain bash (also runs on the macOS bash 3.2 of a dev machine).
# Status: DESIGN; verified by tests/deploy/ops-disk-guard.sh OD6 (fake images in local Docker).
# Change rules: never add `rmi -f` or `system prune`: the previous tag must survive (app-rollback), and volumes
#   (pgdata, caddy-data) must never be touched here.
set -Eeuo pipefail
# shellcheck source-path=SCRIPTDIR source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

keep=" " dry=0
while (($#)); do
  case "$1" in
  --keep)
    [[ "${2:-}" =~ ^[A-Za-z0-9._-]{1,64}$ ]] || lc_die "usage: prune-docker.sh [--keep TAG]... [--dry-run]" 2
    keep+="$2 "
    shift 2
    ;;
  --dry-run) dry=1 && shift ;;
  *) lc_die "usage: prune-docker.sh [--keep TAG]... [--dry-run]" 2 ;;
  esac
done

prefix=${LC_IMAGE_PREFIX:-$(lc_env_file_get "$LC_COMPOSE_ENV" LC_IMAGE_PREFIX || true)}
prefix=${prefix:-lc}
state_dir=${LC_STATE_DIR:-$(lc_env_file_get "$LC_COMPOSE_ENV" LC_STATE_DIR || true)}
log="${state_dir:-/var/lib/live-commerce}/deployments.log"
if [[ ! -r "$log" ]]; then
  lc_info "no $log (not a deploy host, or nothing deployed yet): nothing to prune"
  exit 0
fi

# Running tag (compose.env follows deploy.sh) + the last two distinct tags of deployments.log (column 3, newest last).
keep+="$(lc_env_file_get "$LC_COMPOSE_ENV" IMAGE_TAG || true) "
keep+="$(awk -F'\t' '$3 != "" { for (i = 1; i <= n; i++) if (o[i] == $3) { for (j = i; j < n; j++) o[j] = o[j + 1]; n--; break } o[++n] = $3 }
  END { for (i = n - 1; i <= n; i++) if (i >= 1) printf "%s ", o[i] }' "$log")"
lc_info "keeping tags:$keep"

for img in go admin storefront caddy; do
  repo="$prefix-$img"
  while IFS= read -r tag; do
    [[ -n "$tag" && "$tag" != "<none>" && "$keep" != *" $tag "* ]] || continue
    if ((dry)); then
      lc_info "would remove $repo:$tag"
    elif docker rmi "$repo:$tag" >/dev/null 2>&1; then
      lc_info "removed $repo:$tag"
    else
      lc_warn "kept $repo:$tag (in use by a container)"
    fi
  done < <(docker image ls --format '{{.Tag}}' "$repo" 2>/dev/null || true)
done

# BuildKit cache: Docker >= 29 renamed --keep-storage to --reserved-space (the old name still works but is deprecated).
space_flag=--keep-storage
if grep -q -- '--reserved-space' < <(docker builder prune --help 2>&1); then space_flag=--reserved-space; fi # not `| grep -q`: SIGPIPE under pipefail
if ((dry)); then
  lc_info "would run: docker builder prune -f $space_flag 3GB"
elif ! docker builder prune -f "$space_flag" 3GB >/dev/null; then
  lc_warn "docker builder prune failed (non-fatal)"
else
  lc_info "build cache pruned to <= 3GB"
fi
