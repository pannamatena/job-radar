#!/usr/bin/env bash
# Download the cloud database (the `state` branch) to ./radar.db.
#
#   scripts/state-pull.sh             refuses if your local radar.db has changes never pushed
#   scripts/state-pull.sh --discard   replaces your local radar.db anyway (its changes are lost)
#
# Run it from inside your private config repository. Full guide:
# docs/run-automatically.md in the job-radar project, section "Running job-radar on your computer too".
set -euo pipefail

discard=false
case "${1:-}" in
  "") ;;
  --discard) discard=true ;;
  *) echo "Usage: $0 [--discard]" >&2; exit 2 ;;
esac

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
marker=.radar-state   # line 1: the cloud version we last synced; line 2: the database's checksum then

hash() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

if ! git ls-remote --exit-code --heads origin state >/dev/null 2>&1; then
  echo "There is no 'state' branch in your private repository yet. Nothing to pull." >&2
  exit 1
fi

if [ -f radar.db ] && [ "$discard" = false ]; then
  if [ ! -f "$marker" ]; then
    echo "You already have a radar.db here that was never synced with the cloud copy." >&2
    echo "Pulling would replace it. If you are sure, run: $0 --discard" >&2
    exit 1
  fi
  if [ "$(hash radar.db)" != "$(sed -n 2p "$marker")" ]; then
    echo "Your local radar.db has changes that were never pushed to the cloud." >&2
    echo "Pulling would erase them. Push them first (scripts/state-push.sh), or run: $0 --discard" >&2
    exit 1
  fi
fi

git fetch --quiet origin state
sha=$(git rev-parse FETCH_HEAD)
git show FETCH_HEAD:radar.db > radar.db.tmp
mv radar.db.tmp radar.db
printf '%s\n%s\n' "$sha" "$(hash radar.db)" > "$marker"
echo "Pulled the cloud database (version ${sha:0:8}) to radar.db."
