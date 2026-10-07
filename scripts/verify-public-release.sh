#!/usr/bin/env bash
# Local release gate. CI (.github/workflows/ci.yml) runs the same checks.
#
# Requires TEST_DATABASE_URL pointing at a disposable PostgreSQL database;
# migrations and integration tests run against it. Example:
#   docker run -d --name release-gate-db -e POSTGRES_PASSWORD=postgres -p 55432:5432 postgres:16-alpine
#   TEST_DATABASE_URL='postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable' \
#     bash scripts/verify-public-release.sh

set -euo pipefail

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"
# shellcheck source=lib/gitleaks.sh
source scripts/lib/gitleaks.sh

GOLANGCI_LINT_VERSION=2.10.1
GOVULNCHECK_VERSION=v1.8.0
ACTIONLINT_VERSION=v1.7.12
PLACEHOLDER_API_URL=https://api.example.invalid

current_check=""
publishable_dir=""
step() {
  current_check=$1
  printf '\n==> %s\n' "$1"
}
on_exit() {
  local status=$?
  [[ -n "$publishable_dir" ]] && rm -rf "$publishable_dir"
  if ((status != 0)); then
    printf '\nrelease gate: FAIL (%s)\n' "$current_check" >&2
  fi
}
trap on_exit EXIT

step "prerequisites"
if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
  echo "TEST_DATABASE_URL must point at a disposable PostgreSQL database" >&2
  exit 1
fi
for tool in go npm docker curl tar golangci-lint; do
  command -v "$tool" >/dev/null || { echo "missing required tool: $tool" >&2; exit 1; }
done
golangci-lint version 2>/dev/null | grep -q "version $GOLANGCI_LINT_VERSION" || {
  echo "golangci-lint $GOLANGCI_LINT_VERSION is required" >&2
  exit 1
}

step "repository hygiene"
bash scripts/check-repository-hygiene.sh

step "cloud run deployment policy"
bash scripts/check-cloudrun-config.sh

step "github actions pinned to commit SHAs"
bash scripts/check-action-pins.sh

step "github actions lint (actionlint $ACTIONLINT_VERSION)"
go run "github.com/rhysd/actionlint/cmd/actionlint@$ACTIONLINT_VERSION"

step "backend format"
unformatted=$(cd backend && gofmt -l .)
if [[ -n "$unformatted" ]]; then
  printf '%s\n' "$unformatted" >&2
  exit 1
fi

step "backend vet"
(cd backend && go vet ./...)

step "backend lint (golangci-lint $GOLANGCI_LINT_VERSION)"
(cd backend && golangci-lint run ./...)

step "backend tests with migrations and integration tests"
(cd backend && TEST_DATABASE_URL="$TEST_DATABASE_URL" go test -count=1 ./...)

step "backend vulnerabilities (govulncheck $GOVULNCHECK_VERSION)"
(cd backend && go run "golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSION" ./...)

step "frontend clean install"
(cd frontend && npm ci --no-audit --no-fund)

step "frontend tests"
(cd frontend && CI=true npm test -- --watchAll=false)

step "frontend production build"
(cd frontend && REACT_APP_API_URL=$PLACEHOLDER_API_URL npm run build)

# Runtime dependencies only. Create React App's build/test toolchain
# (devDependencies) carries unfixable advisories and never ships to browsers;
# migrating to Vite is the tracked follow-up.
step "frontend runtime dependency audit (high)"
(cd frontend && npm audit --omit=dev --audit-level=high)

step "backend container build"
docker build -t spotify-backend:release-gate backend

step "frontend container build"
docker build --build-arg REACT_APP_API_URL=$PLACEHOLDER_API_URL -t spotify-frontend:release-gate frontend

step "secret scan (gitleaks $GITLEAKS_VERSION)"
gitleaks_bin=$(ensure_gitleaks)
publishable_dir=$(mktemp -d)
# Scan exactly what Git would publish: tracked plus untracked, non-ignored files.
git ls-files -z --cached --others --exclude-standard |
  while IFS= read -r -d '' path; do [[ -e "$path" ]] && printf '%s\0' "$path"; done |
  tar --null -T - -cf - | tar -xf - -C "$publishable_dir"
scan_secrets "$gitleaks_bin" "$publishable_dir"
scan_secrets "$gitleaks_bin" frontend/build

current_check=""
printf '\nrelease gate: PASS\n'
