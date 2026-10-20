#!/usr/bin/env bash
# Send your local radar.db to the cloud (the `state` branch), but ONLY if the cloud
# database hasn't changed since you pulled it. If a scheduled run happened in the
# meantime, this refuses instead of overwriting it.
#
# Run it from inside your private config repository. Full guide:
# docs/run-automatically.md in the job-radar project, section "Running job-radar on your computer too".
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Guard: only ever run this inside your PRIVATE config repository. The database holds
# personal data, so it must never be pushed to a public repository (such as job-radar's own).
if [ ! -f config.yaml ]; then
  echo "Run this from inside your config folder (the one with config.yaml), not from another repository." >&2
  exit 1
fi
if command -v gh >/dev/null 2>&1 && [ "$(gh repo view --json isPrivate --jq .isPrivate 2>/dev/null || echo true)" = "false" ]; then
  echo "REFUSED: this repository is PUBLIC. Your database must only live in a private repository." >&2
  exit 1
fi
marker=.radar-state

hash() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

if [ ! -f radar.db ]; then
  echo "There is no radar.db here to push." >&2
  exit 1
fi

remote=$(git ls-remote origin refs/heads/state | cut -f1)
if [ -z "$remote" ]; then
  # First time: there is no cloud database yet, so there is nothing to overwrite.
  expect=""
else
  if [ ! -f "$marker" ]; then
    echo "The cloud already has a database, and this radar.db was never pulled from it." >&2
    echo "Pushing would replace the cloud one. Pull first: scripts/state-pull.sh --discard" >&2
    exit 1
  fi
  base=$(sed -n 1p "$marker")
  if [ "$remote" != "$base" ]; then
    echo "REFUSED: the cloud database changed after you pulled it (a scheduled run happened)." >&2
    echo "Pushing now would erase that run, and job-radar would email you old jobs again." >&2
    echo "Your local changes are NOT sent. To redo: scripts/state-pull.sh --discard, then repeat your local run." >&2
    exit 1
  fi
  if [ "$(hash radar.db)" = "$(sed -n 2p "$marker")" ]; then
    echo "radar.db has no changes since you pulled it. Nothing to push."
    exit 0
  fi
  # Keep the cloud copy for one step, so this push can be rolled back.
  git fetch --quiet origin state
  git push --quiet --force origin FETCH_HEAD:refs/heads/state-previous
  expect="$base"
fi

dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
cp radar.db "$dir/"
git -C "$dir" init -q -b state
git -C "$dir" add radar.db
git -C "$dir" -c user.name=job-radar -c user.email=job-radar@users.noreply.github.com commit -q -m "State pushed from my computer"
git -C "$dir" remote add origin "$(git remote get-url origin)"
# --force-with-lease=state:<version> means: overwrite, but only if the cloud is still at <version>.
# With an empty value it means "only if the branch doesn't exist yet".
if ! git -C "$dir" push --quiet --force-with-lease="state:$expect" origin state; then
  echo "REFUSED: the cloud database changed while pushing. Nothing was overwritten." >&2
  echo "Pull again: scripts/state-pull.sh --discard, then repeat your local run." >&2
  exit 1
fi
new=$(git -C "$dir" rev-parse HEAD)
printf '%s\n%s\n' "$new" "$(hash radar.db)" > "$marker"
echo "Pushed radar.db to the cloud (version ${new:0:8})."
