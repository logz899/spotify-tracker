# Cloud Run Deployment

This directory defines the production runtime:

| Resource | Definition | Access |
|---|---|---|
| API service | `api.yaml` | Public HTTPS, scale 0–1, request-based billing |
| Migration job | `migrate-job.yaml` | Private, run manually before each API release |
| Sync job | `sync-job.yaml` | Private, started by one Cloud Scheduler job |
| Image retention | `artifact-cleanup-policy.json` | Keeps the 3 newest images |

All three workloads run the same backend image with different commands
(`/app/server`, `/app/migrate`, `/app/sync`) and different service accounts.
The database is an external Neon Free PostgreSQL database reached over TLS.

Nothing in this repository runs these commands automatically. Every command
below creates or changes billable cloud resources; run them only after
reviewing the target project.

## Configuration

```bash
cp deploy/cloudrun/gcp.env.example deploy/cloudrun/gcp.env
# edit deploy/cloudrun/gcp.env, then load it into the current shell:
set -a; source deploy/cloudrun/gcp.env; set +a
make cloudrun-check-env
```

`gcp.env` holds identifiers only. The four secret values exist only in
Secret Manager:

| Secret | Value |
|---|---|
| `spotify-client-secret` | Spotify application client secret |
| `jwt-secret` | At least 32 random bytes, e.g. `openssl rand -base64 48` |
| `database-url` | Neon **pooled** connection string with `sslmode=require` |
| `token-encryption-key` | Exactly 32 bytes, base64: `openssl rand -base64 32` |

## One-Time Setup

### 1. Enable APIs and create the image repository

```bash
gcloud config set project "$PROJECT_ID"
gcloud services enable run.googleapis.com artifactregistry.googleapis.com \
  secretmanager.googleapis.com cloudscheduler.googleapis.com

gcloud artifacts repositories create "$AR_REPO" \
  --repository-format=docker --location="$REGION"
gcloud artifacts repositories set-cleanup-policies "$AR_REPO" \
  --location="$REGION" --policy=deploy/cloudrun/artifact-cleanup-policy.json
gcloud auth configure-docker "$REGION-docker.pkg.dev"
```

### 2. Create service accounts

```bash
for name in spotify-api spotify-migrate spotify-sync spotify-scheduler; do
  gcloud iam service-accounts create "$name"
done
```

Use the resulting addresses for the `*_SERVICE_ACCOUNT` values in `gcp.env`.
No account receives a project-wide role.

### 3. Create secrets

Each prompt reads the value without echoing it or storing it in shell history.

```bash
for secret in spotify-client-secret jwt-secret database-url token-encryption-key; do
  read -rs -p "$secret: " value; echo
  printf '%s' "$value" | gcloud secrets create "$secret" \
    --replication-policy=automatic --data-file=-
  unset value
done
```

Grant each workload only the secrets it reads:

```bash
for secret in spotify-client-secret jwt-secret database-url token-encryption-key; do
  for account in "$API_SERVICE_ACCOUNT" "$SYNC_SERVICE_ACCOUNT"; do
    gcloud secrets add-iam-policy-binding "$secret" \
      --member="serviceAccount:$account" --role=roles/secretmanager.secretAccessor
  done
done
gcloud secrets add-iam-policy-binding database-url \
  --member="serviceAccount:$MIGRATE_SERVICE_ACCOUNT" \
  --role=roles/secretmanager.secretAccessor
```

When rotating a secret, add the new version and then disable or destroy the
previous one so each secret keeps a single active version.

### 4. Budget alert

```bash
gcloud billing budgets create --billing-account="$BILLING_ACCOUNT_ID" \
  --display-name="spotify-song-rank" --budget-amount=1USD \
  --threshold-rule=percent=0.5 --threshold-rule=percent=0.9 \
  --threshold-rule=percent=1.0
```

Budget alerts only send notifications. They are **not** a spending cap and do
not stop any service.

## Release

```bash
CONFIRM_DEPLOY=yes make cloudrun-deploy
```

The target builds and pushes the image tagged with the current commit, renders
the YAML files with `envsubst`, runs the migration job to completion, then
replaces the API service and the sync job. Rendered files are written to
`deploy/cloudrun/rendered/`, which is gitignored.

Allow public access to the API service only (first release):

```bash
gcloud run services add-iam-policy-binding "$API_SERVICE_NAME" \
  --region="$REGION" --member=allUsers --role=roles/run.invoker
```

Jobs never receive a public binding.

## Scheduler

One Scheduler job starts the sync job every 15 minutes through the Cloud Run
Admin API. Its service account may run only that job:

```bash
gcloud run jobs add-iam-policy-binding "$SYNC_JOB_NAME" --region="$REGION" \
  --member="serviceAccount:$SCHEDULER_SERVICE_ACCOUNT" --role=roles/run.invoker

gcloud scheduler jobs create http "$SCHEDULER_JOB_NAME" \
  --location="$REGION" \
  --schedule="*/15 * * * *" \
  --http-method=POST \
  --uri="https://run.googleapis.com/v2/projects/$PROJECT_ID/locations/$REGION/jobs/$SYNC_JOB_NAME:run" \
  --oauth-service-account-email="$SCHEDULER_SERVICE_ACCOUNT" \
  --oauth-token-scope=https://www.googleapis.com/auth/cloud-platform
```

Overlapping executions are harmless: the sync command takes a PostgreSQL
advisory lock and exits if another run holds it.

## Cost Boundaries

- API: `minScale=0`, `maxScale=1`, CPU only during requests, 30 s timeout.
- Jobs: one task, no retries, finite timeouts (5 min migrate, 12 min sync).
- One Scheduler job (Cloud Scheduler includes a small free allowance of jobs).
- Artifact Registry keeps the 3 newest images and deletes older ones after 7 days.
- Four secrets, one active version each.
- No load balancer, NAT, VM, GKE cluster, Cloud SQL, or Memorystore.
