#!/usr/bin/env bash
#
# Fails if any tracked file holds personal data: anything under private/, a
# local database, an .env file, or the personal BUILD_PLAN.md. This is the CI
# second line of defence behind .gitignore and the pre-commit hook
# (BUILD_PLAN.md §9a point 1, ADR-0018).
#
# It inspects what git is actually tracking, so it catches a file that was
# force-added past .gitignore.

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Patterns that must never appear in a tracked path.
pattern='(^private/|^BUILD_PLAN\.md$|(^|/)\.env($|\.)|\.db(\.bak)?$)'

offenders="$(git ls-files | grep -E "$pattern" || true)"

if [ -n "$offenders" ]; then
	echo "✖ Personal/private files are tracked by git:" >&2
	printf '%s\n' "$offenders" | sed 's/^/    /' >&2
	echo "" >&2
	echo "These must never be committed. Remove them from the index:" >&2
	echo "    git rm --cached <file>   # then commit the removal" >&2
	echo "If one was pushed, treat it as leaked — see SECURITY.md." >&2
	exit 1
fi

echo "✓ No private data tracked by git."
