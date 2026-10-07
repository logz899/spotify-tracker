#!/usr/bin/env bash

set -euo pipefail

repository_root=$(git rev-parse --show-toplevel)
temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/spotify-snapshot-test.XXXXXX")
trap 'rm -rf "$temporary_root"' EXIT

snapshot="$temporary_root/snapshot"
bash "$repository_root/scripts/create-public-snapshot.sh" "$snapshot" >/dev/null

git -C "$snapshot" add --all

required_entrypoints=(
  backend/cmd/api/main.go
  backend/cmd/migrate/main.go
  backend/cmd/sync/main.go
)

failures=()
for entrypoint in "${required_entrypoints[@]}"; do
  if ! git -C "$snapshot" ls-files --error-unmatch "$entrypoint" >/dev/null 2>&1; then
    failures+=("$entrypoint is missing after git add")
  fi
done

if ((${#failures[@]} > 0)); then
  printf 'public snapshot staging test: FAIL\n' >&2
  printf '  - %s\n' "${failures[@]}" >&2
  exit 1
fi

printf 'public snapshot staging test: PASS\n'
