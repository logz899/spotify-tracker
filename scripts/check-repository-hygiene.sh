#!/usr/bin/env bash

set -euo pipefail

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"

forbidden_paths=()

while IFS= read -r -d '' path; do
  [[ -e "$path" || -L "$path" ]] || continue

  case "$path" in
    .env|frontend/.env|helm/.env|helm/.env-secret|backend/api|backend/migrate|backend/requirements.txt)
      forbidden_paths+=("$path")
      ;;
    __pycache__/*|*/__pycache__/*|*.pyc|backend/models/*.py|backend/services/*.py)
      forbidden_paths+=("$path")
      ;;
  esac
done < <(git ls-files --cached --others --exclude-standard -z)

if ((${#forbidden_paths[@]} > 0)); then
  printf '%s\n' "${forbidden_paths[@]}" | LC_ALL=C sort -u >&2
  exit 1
fi

printf '%s\n' 'repository hygiene: PASS'
