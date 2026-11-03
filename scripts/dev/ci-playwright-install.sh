#!/usr/bin/env bash
# Purpose: install the mode's browser dependencies and binaries in CI, touching the network only when the runner image lacks a dependency
#   and the cached .debs cannot supply it (#162: a fresh apt install on every job timed out on a slow mirror, exit=124 x3, before any test ran).
# Depends on: frozen Playwright CLI (`install-deps --dry-run` simulates the install and lists the missing packages, no network),
#   hosted Linux runner tooling (sudo, apt-get, apt-cache, dpkg-deb, Python >=3.11), LC_PLAYWRIGHT_DEBS_DIR = a runner-owned directory that gates.yml restores/saves with actions/cache.
# Used by: gates.yml and fake-network-edge process tests (tests/ci/playwright-deps.test.mjs).
# Invariant: the script exits non-zero unless dpkg reports every package it set out to install as installed; nothing is skipped silently.
set -euo pipefail
[[ "$(uname -s)" == Linux ]] || { echo 'ci-playwright-install: Linux runner required' >&2; exit 2; }
[[ "$*" == chromium || "$*" == 'chromium webkit' ]] || { echo 'ci-playwright-install: invalid browser set' >&2; exit 2; }
# Directly declared package; pnpm does not expose the transitive playwright package at the repository root.
node_bin="$(command -v node)"
cli="$(node -p 'require.resolve("@playwright/test/cli")')"
debs="${LC_PLAYWRIGHT_DEBS_DIR:-}"
started=$SECONDS

# Prints "none", "missing:<pkg> <pkg>..." or "unknown" (the simulation itself failed). Exit codes of the CLI are not trusted: its text is.
dry_run() {
  local out
  out="$("$node_bin" "$cli" install-deps --dry-run "$@" 2>&1 || true)"
  if grep -q 'All system dependencies are installed' <<<"$out"; then echo none
  elif grep -q 'Missing system dependencies' <<<"$out"; then
    local names
    names="$(sed -n '/Missing system dependencies/,$ s/^ *\([a-z0-9][a-z0-9+.:-]*\) *$/\1/p' <<<"$out" | tr '\n' ' ')"
    # A header without a single parsed name is a parser/format problem, never "nothing to install": fall back to the full install.
    if [[ -n "${names// /}" ]]; then echo "missing:$names"; else echo unknown; fi
  else echo unknown; fi
}
# Bounded apt call as root: the monitor can terminate the complete apt process group. Never use --foreground.
apt_bounded() { sudo timeout --kill-after=15s 240s "$@"; }
# P2-3: each cached .deb must match the runner's apt index for its exact package/version. A manifest in the same cache cannot authenticate it.
# Batch the index lookup: repeatedly loading apt's index once per package took ~211s in run 41682937571; all hashes remain mandatory.
verify_debs() {
  python3 - "$debs" <<'PY'
import hashlib, pathlib, re, subprocess, sys

try:
    files = sorted(pathlib.Path(sys.argv[1]).glob('*.deb'))
    if not files:
        raise ValueError('empty cache')
    entries = []
    for file in files:
        metadata = subprocess.run(['dpkg-deb', '-f', str(file), 'Package', 'Version'], check=True, capture_output=True, text=True, timeout=30).stdout
        fields = dict(line.split(': ', 1) for line in metadata.splitlines() if ': ' in line)
        pkg, version = fields.get('Package', ''), fields.get('Version', '')
        if not re.fullmatch(r'[a-z0-9][a-z0-9+.-]*', pkg) or not re.fullmatch(r'[A-Za-z0-9.+:~_-]+', version):
            raise ValueError(f'invalid package metadata: {file.name}')
        entries.append((file, pkg, version))
    selectors = list(dict.fromkeys(f'{pkg}={version}' for _, pkg, version in entries))
    index = subprocess.run(['apt-cache', 'show', *selectors], check=True, capture_output=True, text=True, timeout=120).stdout
    hashes = {}
    for paragraph in re.split(r'\n\s*\n', index):
        fields = dict(line.split(': ', 1) for line in paragraph.splitlines() if ': ' in line)
        key = fields.get('Package'), fields.get('Version')
        sha = fields.get('SHA256', '')
        if re.fullmatch(r'[0-9a-fA-F]{64}', sha):
            hashes.setdefault(key, set()).add(sha.lower())
    for file, pkg, version in entries:
        with file.open('rb') as stream:
            digest = hashlib.file_digest(stream, 'sha256').hexdigest()
        if digest not in hashes.get((pkg, version), set()):
            raise ValueError(f'{file.name} not verified against apt index')
except (OSError, ValueError, subprocess.SubprocessError) as error:
    print(f'ci-playwright-install: cache verification failed: {error}', file=sys.stderr)
    sys.exit(1)
PY
}

state="$(dry_run "$@")"
echo "ci-playwright-install: browsers=$* state=${state%%:*}"
[[ "$state" != missing:* ]] || echo "ci-playwright-install: runner image lacks: ${state#missing:}"
if [[ "$state" == none ]]; then
  echo 'ci-playwright-install: runner image already provides every dependency; no apt call'
else
  if [[ "$state" == missing:* ]]; then
    # shellcheck disable=SC2206
    missing=(${state#missing:})
    [[ -z "$debs" ]] || mkdir -p "$debs"
    # 1. Offline from the cached .debs (no index refresh, no download); any miss or partial cache falls through to the network.
    offline=0
    if [[ -n "$debs" ]] && compgen -G "$debs/*.deb" >/dev/null; then
      if verify_debs; then
        # No network involved, so a short bound (the whole step stays within its 15 min budget even when this fails).
        if sudo timeout --kill-after=15s 60s apt-get install -y --no-install-recommends --no-download -o "Dir::Cache::archives=$debs" "${missing[@]}"; then offline=1; fi
      else
        echo 'ci-playwright-install: discarding the cached .debs; using the network path' >&2
        rm -f "$debs"/*.deb
      fi
    fi
    if [[ "$offline" == 1 ]]; then
      echo "ci-playwright-install: dependencies installed offline from verified cache (${#missing[@]} packages)"
    else
      rc=1
      for attempt in 1 2 3; do
        echo "ci-playwright-install: dependencies attempt=$attempt/3 browsers=$* limit=240s kill-after=15s"
        # ONE 240s bound covers update + install together (P2-1): the cap per attempt is not doubled.
        if apt_bounded bash -c 'apt-get update && exec apt-get install -y --no-install-recommends "$@"' _ -o APT::Keep-Downloaded-Packages=true ${debs:+-o "Dir::Cache::archives=$debs"} "${missing[@]}"; then rc=0; break; else rc=$?; fi
        echo "ci-playwright-install: dependencies attempt=$attempt exit=$rc elapsed=$((SECONDS-started))s" >&2
        [[ "$attempt" == 3 ]] || sleep 10
      done
      [[ "$rc" == 0 ]] || { echo "ci-playwright-install: dependencies failed after 3 attempts (exit=$rc)" >&2; exit "$rc"; }
      if [[ -n "$debs" ]]; then
        sudo chown -R "$(id -u):$(id -g)" "$debs"; rm -rf "$debs/partial" "$debs/lock"
        [[ -z "${GITHUB_OUTPUT:-}" ]] || echo 'debs=downloaded' >> "$GITHUB_OUTPUT"
      fi
    fi
  else
    # The simulation could not list the delta (e.g. no apt index): install the full Playwright set exactly as before.
    rc=1
    for attempt in 1 2 3; do
      echo "ci-playwright-install: dependencies attempt=$attempt/3 browsers=$* limit=240s kill-after=15s"
      if apt_bounded "$node_bin" "$cli" install-deps "$@"; then rc=0; break; else rc=$?; fi
      echo "ci-playwright-install: dependencies attempt=$attempt exit=$rc elapsed=$((SECONDS-started))s" >&2
      [[ "$attempt" == 3 ]] || sleep 10
    done
    [[ "$rc" == 0 ]] || { echo "ci-playwright-install: dependencies failed after 3 attempts (exit=$rc)" >&2; exit "$rc"; }
  fi
  # Whatever path ran, every package we set out to install must now be present (dpkg state, not a second simulation: after
  # `apt-get update` a simulation also lists mere upgrades of installed packages as "Inst"); otherwise the job is red.
  if [[ "$state" == missing:* ]]; then
    bad=()
    for pkg in "${missing[@]}"; do
      [[ "$(dpkg-query -W -f='${db:Status-Status}' "$pkg" 2>/dev/null || true)" == installed ]] || bad+=("$pkg")
    done
    [[ ${#bad[@]} == 0 ]] || { echo "ci-playwright-install: dependencies still missing after install: ${bad[*]}" >&2; exit 1; }
  fi
fi
# Browser downloads use the user's versioned cache; system dependencies are never inferred from a cache hit.
pnpm exec playwright install "$@"
echo "ci-playwright-install: browsers=$* elapsed=$((SECONDS-started))s"
