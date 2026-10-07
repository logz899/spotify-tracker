#!/usr/bin/env bash
# Creates a sanitized, new-history copy of the publishable project.
#
# Usage: bash scripts/create-public-snapshot.sh DESTINATION_DIR
#
# Copies only allowlisted paths from the working tree (tracked plus untracked,
# non-ignored files), initializes a fresh Git repository with no commits and no
# remote, then verifies hygiene and scans for secrets. It never commits, adds a
# remote, or pushes; those are deliberate human actions.

set -euo pipefail

if (($# != 1)); then
  echo "usage: $0 DESTINATION_DIR" >&2
  exit 2
fi

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"
repository_root=$(pwd -P)
# shellcheck source=lib/gitleaks.sh
source scripts/lib/gitleaks.sh

# Publishable top-level paths. Agent plans, local notes, and editor state stay
# in the private repository.
allowlist=(
  .env.example
  .github
  .gitignore
  .golangci.yaml
  .pre-commit-config.yaml
  Makefile
  README.md
  SECURITY.md
  backend
  deploy
  docker-compose.yml
  frontend
  scripts
)

destination=$1
if [[ -e "$destination" && ! -d "$destination" ]]; then
  echo "destination exists and is not a directory" >&2
  exit 1
fi
created_destination=false
if [[ ! -e "$destination" ]]; then
  mkdir -p "$destination"
  created_destination=true
fi
destination=$(cd "$destination" && pwd -P)

reject() {
  [[ "$created_destination" == true ]] && rmdir "$destination" 2>/dev/null
  echo "$1" >&2
  exit 1
}

case "$destination/" in
  "$repository_root"/*) reject "destination must be outside the repository" ;;
esac
case "$repository_root/" in
  "$destination"/*) reject "destination must not contain the repository" ;;
esac
if [[ -n "$(ls -A "$destination")" ]]; then
  reject "destination must be empty"
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "warning: working tree has uncommitted changes; they are included in the snapshot" >&2
fi

is_allowlisted() {
  local path=$1 entry
  for entry in "${allowlist[@]}"; do
    [[ "$path" == "$entry" || "$path" == "$entry"/* ]] && return 0
  done
  return 1
}

is_denied() {
  case "$1" in
    .env.example|*/.env.example) return 1 ;;
    .env|.env.*|*/.env|*/.env.*|gcp.env|*/gcp.env) return 0 ;;
    deploy/cloudrun/rendered/*) return 0 ;;
    *.pem|*.key|*.p12|*service-account*.json|*credentials*.json) return 0 ;;
  esac
  return 1
}

copied=0
while IFS= read -r -d '' path; do
  [[ -f "$path" || -L "$path" ]] || continue
  is_allowlisted "$path" || continue
  if is_denied "$path"; then
    echo "skipping denied path: $path" >&2
    continue
  fi
  mkdir -p "$destination/$(dirname "$path")"
  cp -P "$path" "$destination/$path"
  copied=$((copied + 1))
done < <(git ls-files -z --cached --others --exclude-standard)

git -C "$destination" init --quiet --initial-branch=main

# Verify the snapshot before handing it over.
failures=()
[[ -z "$(git -C "$destination" remote)" ]] || failures+=("snapshot has a remote")
if git -C "$destination" rev-parse --verify --quiet HEAD >/dev/null; then
  failures+=("snapshot already has commits")
fi
if [[ -n "$(find "$destination" -mindepth 2 -name .git -print -quit)" ]]; then
  failures+=("snapshot contains nested .git directories")
fi
for forbidden in .agent .superpowers CLAUDE.md AGENTS.md helm deploy/cloudbuild.yaml \
  frontend/node_modules frontend/build .env; do
  [[ -e "$destination/$forbidden" ]] && failures+=("snapshot contains $forbidden")
done
if [[ -n "$(find "$destination" -path "$destination/.git" -prune -o \
  \( -name '__pycache__' -o -name '*.pyc' -o -name '.DS_Store' \) -print -quit)" ]]; then
  failures+=("snapshot contains caches or OS metadata")
fi
# Split so this script does not match its own check.
private_backup_marker="spotify-agent-""backup"
if grep -rIl --exclude-dir=.git -e "$private_backup_marker" "$destination" >/dev/null 2>&1; then
  failures+=("snapshot references a private backup path")
fi
(cd "$destination" && bash scripts/check-repository-hygiene.sh >/dev/null) ||
  failures+=("snapshot fails repository hygiene")

if ((${#failures[@]} > 0)); then
  printf 'public snapshot: FAIL\n' >&2
  printf '  - %s\n' "${failures[@]}" >&2
  exit 1
fi

gitleaks_bin=$(ensure_gitleaks)
scan_secrets "$gitleaks_bin" "$destination"

printf 'public snapshot: PASS (%d files, no commits, no remote)\n' "$copied"
printf 'location: %s\n' "$destination"
printf 'next: review, then commit and add the new remote yourself.\n'
