#!/usr/bin/env bash
# Fails unless every GitHub Actions `uses:` reference is pinned to a full
# commit SHA with its human-readable version recorded in a trailing comment.

set -euo pipefail

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"

shopt -s nullglob
workflows=(.github/workflows/*.yml .github/workflows/*.yaml)
if ((${#workflows[@]} == 0)); then
  echo "no workflows found in .github/workflows" >&2
  exit 1
fi

failures=0
for workflow in "${workflows[@]}"; do
  while IFS= read -r line; do
    reference=${line#*uses:}
    reference=${reference%%#*}
    reference=$(printf '%s' "$reference" | tr -d "[:space:]\"'")
    # Local actions (./path) and docker:// images are not version-resolved.
    [[ "$reference" == ./* || "$reference" == docker://* ]] && continue
    if [[ ! "$reference" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$ ]] ||
      [[ ! "$line" =~ \#[[:space:]]*v[0-9] ]]; then
      echo "$workflow: unpinned action: $reference" >&2
      failures=$((failures + 1))
    fi
  done < <(grep -E '^[[:space:]-]*uses:' "$workflow")
done

if ((failures > 0)); then
  exit 1
fi
printf '%s\n' 'action pins: PASS'
