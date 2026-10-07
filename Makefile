# ──────────────────────────────────────────────────────────────────────────────
# Spotify Song Rank — root Makefile
# Usage: make <target>
#
# Cloud Run targets read deploy/cloudrun/gcp.env (copy from gcp.env.example).
# Targets that change cloud resources require CONFIRM_DEPLOY=yes.
# ──────────────────────────────────────────────────────────────────────────────

GCP_ENV := deploy/cloudrun/gcp.env
-include $(GCP_ENV)
export

IMAGE_TAG    ?= $(shell git rev-parse --short HEAD)
IMAGE        := $(REGION)-docker.pkg.dev/$(PROJECT_ID)/$(AR_REPO)/backend:$(IMAGE_TAG)
RENDER_DIR   := deploy/cloudrun/rendered
RENDER_VARS  := $${IMAGE} $${API_SERVICE_NAME} $${MIGRATE_JOB_NAME} $${SYNC_JOB_NAME} \
                $${API_SERVICE_ACCOUNT} $${MIGRATE_SERVICE_ACCOUNT} $${SYNC_SERVICE_ACCOUNT} \
                $${FRONTEND_URL} $${CORS_ALLOWED_ORIGINS} $${SPOTIFY_CLIENT_ID} \
                $${SPOTIFY_REDIRECT_URI} $${JWT_ISSUER} $${JWT_AUDIENCE}
REQUIRED_VARS := PROJECT_ID REGION AR_REPO API_SERVICE_NAME MIGRATE_JOB_NAME SYNC_JOB_NAME \
                 API_SERVICE_ACCOUNT MIGRATE_SERVICE_ACCOUNT SYNC_SERVICE_ACCOUNT \
                 FRONTEND_URL CORS_ALLOWED_ORIGINS SPOTIFY_CLIENT_ID SPOTIFY_REDIRECT_URI \
                 JWT_ISSUER JWT_AUDIENCE

export IMAGE

.PHONY: help \
        local-up local-down \
        test test-backend test-frontend check \
        cloudrun-check cloudrun-check-env cloudrun-render \
        docker-build-backend \
        cloudrun-confirm cloudrun-push cloudrun-migrate cloudrun-deploy

# ── Default ────────────────────────────────────────────────────────────────────
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo ''
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Local development:'
	@awk 'BEGIN {FS = ":.*?## "} /^local.*:.*?## / {printf "  %-28s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Validation (no cloud access):'
	@awk 'BEGIN {FS = ":.*?## "} /^(test|check|cloudrun-check|cloudrun-render|docker).*:.*?## / {printf "  %-28s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Cloud Run release (requires CONFIRM_DEPLOY=yes):'
	@awk 'BEGIN {FS = ":.*?## "} /^(cloudrun-push|cloudrun-migrate|cloudrun-deploy).*:.*?## / {printf "  %-28s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'One-time cloud setup is documented in deploy/cloudrun/README.md.'

# ── Local development ──────────────────────────────────────────────────────────
local-up: ## Start the full stack locally with Docker Compose
	docker compose up --build

local-down: ## Stop and remove local Docker Compose containers and volumes
	docker compose down -v

# ── Validation ─────────────────────────────────────────────────────────────────
test: test-backend test-frontend ## Run backend and frontend tests

test-backend: ## Run backend tests and vet
	cd backend && go test ./... && go vet ./...

test-frontend: ## Run frontend tests
	cd frontend && CI=true npm test -- --watchAll=false

check: cloudrun-check ## Run repository hygiene and deployment policy checks
	bash scripts/check-repository-hygiene.sh

cloudrun-check: ## Statically verify Cloud Run definitions (no cloud access)
	bash scripts/check-cloudrun-config.sh

docker-build-backend: ## Build the backend image locally for Cloud Run (linux/amd64)
	docker build --platform linux/amd64 -t spotify-backend:$(IMAGE_TAG) ./backend

cloudrun-check-env: ## Verify deploy/cloudrun/gcp.env defines every required value
	@test -f $(GCP_ENV) || (echo "ERROR: $(GCP_ENV) not found. Copy deploy/cloudrun/gcp.env.example." && exit 1)
	@for var in $(REQUIRED_VARS); do \
		eval "value=\$${$$var}"; \
		test -n "$$value" || { echo "ERROR: $$var is not set in $(GCP_ENV)"; exit 1; }; \
	done
	@echo "$(GCP_ENV): all required values set"

cloudrun-render: cloudrun-check-env ## Render Cloud Run YAML into deploy/cloudrun/rendered/
	@mkdir -p $(RENDER_DIR)
	@for file in api migrate-job sync-job; do \
		envsubst '$(RENDER_VARS)' < deploy/cloudrun/$$file.yaml > $(RENDER_DIR)/$$file.yaml; \
	done
	@echo "Rendered $(RENDER_DIR)/{api,migrate-job,sync-job}.yaml for image $(IMAGE)"

# ── Cloud Run release ──────────────────────────────────────────────────────────
cloudrun-confirm:
	@test "$(CONFIRM_DEPLOY)" = "yes" || \
		(echo "Refusing to change cloud resources. Re-run with CONFIRM_DEPLOY=yes." && exit 1)

cloudrun-push: cloudrun-confirm cloudrun-check cloudrun-check-env ## Build and push the backend image
	docker build --platform linux/amd64 -t $(IMAGE) ./backend
	docker push $(IMAGE)

cloudrun-migrate: cloudrun-confirm cloudrun-render ## Apply migrations with the private migration job
	gcloud run jobs replace $(RENDER_DIR)/migrate-job.yaml --region=$(REGION) --project=$(PROJECT_ID)
	gcloud run jobs execute $(MIGRATE_JOB_NAME) --wait --region=$(REGION) --project=$(PROJECT_ID)

cloudrun-deploy: cloudrun-push cloudrun-migrate ## Push image, migrate, then deploy the API and sync job
	gcloud run services replace $(RENDER_DIR)/api.yaml --region=$(REGION) --project=$(PROJECT_ID)
	gcloud run jobs replace $(RENDER_DIR)/sync-job.yaml --region=$(REGION) --project=$(PROJECT_ID)
