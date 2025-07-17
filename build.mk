# ─── Build Configuration ────────────────────────────────────────────────────────

# Docker/Build Configuration
PROD_REPO     := registry.e2enetworks.net/aimle2e
TEST_REPO     := nirnaye2e
IMAGE_NAME    ?= admission-webhook
TAG           ?= v2.2
ENV           ?= test
REPO          := $(if $(filter production,$(ENV)),$(PROD_REPO),$(TEST_REPO))
IMAGE         := $(REPO)/$(IMAGE_NAME):$(TAG)
DOCKERFILE    := $(if $(filter production,$(ENV)),Dockerfile,Dockerfile.testenv)
PLATFORM      := linux/amd64
CODE_DIR      := code

# Remote Deployment Configuration
ZIP_FILE      := data.zip
# TARGET_IP can be changed depending on current available build node
# This IP represents the remote build server used for consistent environment isolation
TARGET_IP     := 164.52.198.93
TARGET_DIR    := /root/webhooks
NODE_USER     := root

# ─── Build Targets ──────────────────────────────────────────────────────────────

.PHONY: test archive clean-remote deploy remove-local remote-sync \
        build-image-remote push-image-remote remote-build-and-push

## Run all Go tests in $(CODE_DIR).
test: ## Run all Go tests in $(CODE_DIR).
	@echo "🔍 Running Go tests in $(CODE_DIR)…"
	cd $(CODE_DIR) && go test -v ./...

format: ## Format Go code in $(CODE_DIR).
	@echo "📝 Formatting Go code in $(CODE_DIR)…"
	cd $(CODE_DIR) && go fmt ./...

## Create zip archive of project (excluding unwanted files).
archive: ## Create zip archive of project.
	@echo "🗜️  Creating archive $(ZIP_FILE)…"
	zip -r $(ZIP_FILE) ./ \
	    -x "*.git*" "*.DS_Store" "Makefile" "README.md" "LICENSE" "docs/*" \
	       "tls/*" "deploy/*"

## Remove and recreate remote directory.
clean-remote: ## Remove and recreate remote directory.
	@echo "🧹 Cleaning remote directory $(TARGET_DIR)…"
	ssh $(NODE_USER)@$(TARGET_IP) "\
	  rm -rf $(TARGET_DIR) && \
	  mkdir -p $(TARGET_DIR) && \
	  echo '✔️  Remote directory cleaned.' \
	"

## Deploy project archive to remote.
deploy: archive clean-remote ## Deploy archive to remote server.
	@echo "🚀 Deploying archive to remote…"
	scp $(ZIP_FILE) $(NODE_USER)@$(TARGET_IP):$(TARGET_DIR)
	ssh $(NODE_USER)@$(TARGET_IP) "\
	  unzip -o $(TARGET_DIR)/$(ZIP_FILE) -d $(TARGET_DIR) && \
	  rm -f $(TARGET_DIR)/$(ZIP_FILE) && \
	  echo '✔️  Files unpacked on remote.' \
	"

## Remove local zip archive.
remove-local: ## Remove local zip archive.
	@echo "🗑️  Removing local archive…"
	rm -f $(ZIP_FILE)

## Clean up locally and sync to remote.
remote-sync: deploy remove-local ## Clean up locally and sync to remote.
	@echo "✔️  Local cleanup and remote sync complete."

## Build Docker image on remote server.
build-image-remote: test remote-sync ## Build Docker image on remote (ENV=production uses Dockerfile+aimle2e, others use Dockerfile.testenv+nirnaye2e).
	@echo "🔨 Building Docker image $(IMAGE) on remote using $(DOCKERFILE) (ENV=$(ENV))…"
	ssh $(NODE_USER)@$(TARGET_IP) "\
	  cd $(TARGET_DIR) && \
	  docker build --platform=$(PLATFORM) -f $(DOCKERFILE) -t \"$(IMAGE)\" . \
	"

## Push Docker image from remote server.
push-image-remote: test build-image-remote ## Push Docker image from remote (ENV=production uses Dockerfile+aimle2e, others use Dockerfile.testenv+nirnaye2e).
	@echo "📤 Pushing Docker image $(IMAGE) from remote…"
	ssh $(NODE_USER)@$(TARGET_IP) "docker push \"$(IMAGE)\""
	@echo "🎉 Remote build and push complete."

## Build and push Docker image remotely.
remote-build-and-push: test build-image-remote push-image-remote ## Build and push Docker image remotely (ENV=production uses Dockerfile+aimle2e, others use Dockerfile.testenv+nirnaye2e).
	@echo "✔️  Remote Docker image build and push complete."
