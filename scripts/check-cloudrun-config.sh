#!/usr/bin/env bash
# Static policy check for Cloud Run deployment definitions. It never calls
# gcloud; it only inspects tracked files so it is safe to run anywhere.

set -euo pipefail

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"

deploy_dir=deploy/cloudrun
failures=()

fail() {
  failures+=("$1")
}

require_file() {
  [[ -f "$1" ]] || fail "missing $1"
}

# require_pattern <file> <extended-regex> <message>
require_pattern() {
  [[ -f "$1" ]] || return 0
  grep -Eq -- "$2" "$1" || fail "$1: $3"
}

# forbid_pattern <file> <extended-regex> <message>
forbid_pattern() {
  [[ -f "$1" ]] || return 0
  if grep -Eq -- "$2" "$1"; then
    fail "$1: $3"
  fi
}

api=$deploy_dir/api.yaml
migrate_job=$deploy_dir/migrate-job.yaml
sync_job=$deploy_dir/sync-job.yaml
cleanup_policy=$deploy_dir/artifact-cleanup-policy.json
readme=$deploy_dir/README.md
env_example=$deploy_dir/gcp.env.example

for file in "$api" "$migrate_job" "$sync_job" "$cleanup_policy" "$readme" "$env_example"; do
  require_file "$file"
done

# API: request-based billing, scale 0..1, finite timeout, explicit command.
require_pattern "$api" "^kind: Service$" "must be a Cloud Run Service"
require_pattern "$api" "autoscaling\.knative\.dev/minScale: ['\"]0['\"]" "minScale must be 0"
require_pattern "$api" "autoscaling\.knative\.dev/maxScale: ['\"]1['\"]" "maxScale must be 1"
require_pattern "$api" "run\.googleapis\.com/cpu-throttling: ['\"]true['\"]" "must use request-based billing (cpu-throttling true)"
forbid_pattern "$api" "run\.googleapis\.com/(startup-cpu-boost|minScale)" "must not enable always-on CPU features"
require_pattern "$api" "timeoutSeconds: [0-9]+" "must set a finite request timeout"
require_pattern "$api" "/app/server" "must run /app/server"
require_pattern "$api" "name: APP_ENV" "must set APP_ENV"
require_pattern "$api" "value: production" "must run with APP_ENV=production"

# Jobs: explicit commands, finite timeouts, no retries that duplicate work.
require_pattern "$migrate_job" "^kind: Job$" "must be a Cloud Run Job"
require_pattern "$migrate_job" "/app/migrate" "must run /app/migrate"
require_pattern "$sync_job" "^kind: Job$" "must be a Cloud Run Job"
require_pattern "$sync_job" "/app/sync" "must run /app/sync"
for job in "$migrate_job" "$sync_job"; do
  require_pattern "$job" "timeoutSeconds: [0-9]+" "must set a finite task timeout"
  require_pattern "$job" "maxRetries: 0" "must not retry automatically"
  require_pattern "$job" "memory: 512Mi" "must use the Cloud Run gen2 minimum memory for 1 CPU"
  forbid_pattern "$job" "allUsers|allAuthenticatedUsers" "jobs must not be publicly invocable"
done

# Secrets: only Secret Manager references, never literal values.
for file in "$api" "$migrate_job" "$sync_job"; do
  [[ -f "$file" ]] || continue
  require_pattern "$file" "name: DATABASE_URL" "must receive DATABASE_URL"
  # DATABASE_URL must be followed by a secretKeyRef, not an inline value.
  if ! awk '/name: DATABASE_URL/{getline; if ($0 ~ /valueFrom:/) found=1} END{exit !found}' "$file"; then
    fail "$file: DATABASE_URL must come from Secret Manager"
  fi
  require_pattern "$file" "name: database-url" "must reference the database-url secret"
  forbid_pattern "$file" "(postgres|postgresql|pgx5)://" "must not contain a database URL"
  require_pattern "$file" "serviceAccountName: \\\$\{[A-Z_]+_SERVICE_ACCOUNT\}" "service account must be a placeholder"
  forbid_pattern "$file" "@[a-z0-9-]+\.iam\.gserviceaccount\.com" "must not hard-code a service account"
  forbid_pattern "$file" "image: [a-z0-9-]+-docker\.pkg\.dev/[a-z]" "image must be a placeholder"
done

# Every secret-named variable must be a secretKeyRef.
for file in "$api" "$migrate_job" "$sync_job"; do
  [[ -f "$file" ]] || continue
  if ! awk '
    /name: (SPOTIFY_CLIENT_SECRET|JWT_SECRET|TOKEN_ENCRYPTION_KEY|DATABASE_URL)$/ {
      name=$0; getline
      if ($0 !~ /valueFrom:/) { print name; bad=1 }
    }
    END { exit bad }' "$file" >/dev/null; then
    fail "$file: secret variables must use secretKeyRef"
  fi
done

# The API and sync job load full config; migration needs only the database.
for secret in spotify-client-secret jwt-secret database-url token-encryption-key; do
  require_pattern "$api" "name: $secret" "must reference secret $secret"
done
forbid_pattern "$migrate_job" "name: (spotify-client-secret|jwt-secret|token-encryption-key)$" "migration job needs only database-url"

# Artifact Registry cleanup keeps only a few recent images.
require_pattern "$cleanup_policy" "\"keepCount\": [1-9]" "must keep a small number of recent images"
require_pattern "$cleanup_policy" "\"type\": \"Delete\"" "must delete older images"

# Scheduler: one authenticated job, every 15 minutes.
require_pattern "$readme" "gcloud scheduler jobs create http" "must document the Scheduler job"
require_pattern "$readme" "\*/15 \* \* \* \*" "Scheduler must run every 15 minutes"
require_pattern "$readme" "--oauth-service-account-email" "Scheduler must authenticate with a service account"
# Only the API service may be public; allUsers appears once, for that binding.
require_pattern "$readme" "gcloud run services add-iam-policy-binding" "must document the public API binding"
if [[ -f "$readme" ]] && (( $(grep -c "allUsers" "$readme") > 1 )); then
  fail "$readme: only the API service may grant allUsers"
fi
forbid_pattern "$readme" "gcloud run jobs add-iam-policy-binding.*allUsers" "jobs must stay private"
forbid_pattern "$readme" "@[a-z0-9-]+\.iam\.gserviceaccount\.com" "must not hard-code a service account"

# Placeholder environment file must not carry real-looking values.
forbid_pattern "$env_example" "(postgres|postgresql)://" "must not contain a database URL"
forbid_pattern "$env_example" "SECRET=.+" "must not contain secret values"

# Legacy fixed-cost deployment must be gone.
for path in helm deploy/cloudbuild.yaml gcp.env.example; do
  [[ -e "$path" ]] && fail "legacy deployment path still present: $path"
done
if git ls-files --cached --others --exclude-standard -- deploy | xargs -r grep -lE "kind: (StatefulSet|Ingress)|image: (redis|postgres)" 2>/dev/null | grep -q .; then
  fail "deploy/ must not define StatefulSets, Ingress, Redis, or PostgreSQL"
fi
if grep -Eq "helm|kubectl|gcloud container|GKE" Makefile; then
  fail "Makefile still references GKE/Helm tooling"
fi

if ((${#failures[@]} > 0)); then
  printf 'cloud run config: FAIL\n' >&2
  printf '  - %s\n' "${failures[@]}" >&2
  exit 1
fi

printf '%s\n' 'cloud run config: PASS'
