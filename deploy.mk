# ─── Deployment Configuration ──────────────────────────────────────────────────
#
# Usage Examples:
#   make k8s-deploy-update ENV=thor         # Deploy with THOR RabbitMQ URL
#   make k8s-deploy-update ENV=loki         # Deploy with LOKI RabbitMQ URL
#   make k8s-deploy-update ENV=groot        # Deploy with GROOT RabbitMQ URL
#   make k8s-deploy-update ENV=$(PRODUCTION_ENV) # Deploy to production
#   make update-rabbitmq-url ENV=thor       # Only update RabbitMQ URL in secret file
#

# Kubernetes Configuration
K8S_NAMESPACE := channelog
ENV           ?= test
PRODUCTION_ENV := production

# RabbitMQ URLs for different environments
RABBITMQ_URL_THOR  := amqp://gpu:gpuuser@172.16.231.14:5672/gpu
RABBITMQ_URL_LOKI  := amqp://gpu:gpuuser@172.16.232.15:5672/gpu
RABBITMQ_URL_GROOT := amqp://gpu:gpuuser@172.16.230.14:5672/gpu

# Determine RabbitMQ URL based on ENV
RABBITMQ_URL := $(if $(filter thor,$(ENV)),$(RABBITMQ_URL_THOR),\
                $(if $(filter loki,$(ENV)),$(RABBITMQ_URL_LOKI),\
                $(if $(filter groot,$(ENV)),$(RABBITMQ_URL_GROOT),\
                $(RABBITMQ_URL_THOR))))

# Utility Configuration
SED_INPLACE   := $(shell if sed --version >/dev/null 2>&1; then echo "sed -i"; else echo "sed -i ''"; fi)

# Determine deployment directory and files based on ENV
DEPLOY_DIR    := $(if $(filter $(PRODUCTION_ENV),$(ENV)),deploy,deploy/testenv)
SECRET_FILE   := $(if $(filter $(PRODUCTION_ENV),$(ENV)),$(DEPLOY_DIR)/secret.yaml,$(DEPLOY_DIR)/secret_test.yaml)
CONFIG_FILE   := $(DEPLOY_DIR)/config.yaml
DEPLOYMENT_FILE := $(DEPLOY_DIR)/deployment.yaml

# ─── Deployment Targets ─────────────────────────────────────────────────────────

.PHONY: cert-generate cert-update cert-refresh \
        k8s-deploy-update k8s-deploy-full delete-all \
        update-rabbitmq-url

## Update RabbitMQ URL in secret file based on ENV.
update-rabbitmq-url: ## Update RabbitMQ URL in secret file (ENV=thor/loki/groot).
	@echo "🐰 Updating RabbitMQ URL for environment: $(ENV)"
	@echo "🔧 Using RabbitMQ URL: $(RABBITMQ_URL)"
	@echo "🔧 Using secret file: $(SECRET_FILE)"
	@if [ -f $(SECRET_FILE) ]; then \
	  $(SED_INPLACE) -E \
	    "s|RABBITMQ_URL:[[:space:]]*.*|RABBITMQ_URL: $(RABBITMQ_URL)|" \
	    $(SECRET_FILE); \
	  echo "✔️  RabbitMQ URL updated in $(SECRET_FILE)"; \
	else \
	  echo "❌ Error: Secret file $(SECRET_FILE) not found"; \
	  exit 1; \
	fi

## Generate new TLS certificates.
cert-generate: ## Generate new TLS certificates.
	@echo "🔐 Generating new TLS certificates…"
	chmod +x scripts/generate-certs.sh
	scripts/generate-certs.sh

## Update deployment manifests with new certificates.
cert-update: ## Update deployment manifests with new certificates.
	@echo "📝 Updating deployment manifests with new certs…"
	chmod +x scripts/update-deploy.sh
	scripts/update-deploy.sh $(DEPLOY_DIR)

## Regenerate certificates and update manifests.
cert-refresh: cert-generate cert-update ## Regenerate certs and update manifests.
	@echo "✔️  Certificates regenerated and manifests updated."

## Patch deployment.yaml with new image and rollout.
k8s-deploy-update: update-rabbitmq-url ## Patch deployment.yaml with new image and rollout (ENV=$(PRODUCTION_ENV) uses deploy/, others use deploy/testenv/).
	@echo "🔄 Patching $(DEPLOYMENT_FILE) → image $(IMAGE)…"
	@$(SED_INPLACE) -E \
		"s|(image:[[:space:]]*)[^[:space:]]+/$(IMAGE_NAME):[[:alnum:]._-]+|\\1$(IMAGE)|" \
		$(DEPLOYMENT_FILE)

	@echo "🔧 Using deployment directory: $(DEPLOY_DIR) (ENV=$(ENV))"
	@echo "🔧 Using secret file: $(SECRET_FILE)"
	@if [ -f $(SECRET_FILE) ]; then \
	  kubectl apply -f $(SECRET_FILE); \
	else \
	  echo "❌ Error: Secret file $(SECRET_FILE) not found"; \
	  exit 1; \
	fi
	kubectl apply -f $(CONFIG_FILE)
	kubectl apply -f $(DEPLOYMENT_FILE)
	kubectl -n $(K8S_NAMESPACE) rollout restart deployment/channelog

	@echo "⏳ Waiting for rollout to finish…"
	kubectl rollout status deployment/channelog -n $(K8S_NAMESPACE)

## Regenerate certs, patch deployment, and rollout.
k8s-deploy-full: cert-refresh k8s-deploy-update ## Regenerate certs, patch deployment, and rollout (ENV=$(PRODUCTION_ENV) uses deploy/, others use deploy/testenv/).
	@echo "✔️  Certificates regenerated, deployment patched, and rollout initiated."

## Delete all resources in namespace.
delete-all: ## Delete all resources in the k8s namespace (ENV=$(PRODUCTION_ENV) uses deploy/, others use deploy/testenv/).
	@echo "🗑️  Deleting all resources in namespace $(K8S_NAMESPACE)…"
	@echo "🔧 Using deployment directory: $(DEPLOY_DIR) (ENV=$(ENV))"
	kubectl delete -f $(CONFIG_FILE) || true
	kubectl delete -n $(K8S_NAMESPACE) -f $(DEPLOYMENT_FILE) || true
	@if [ -f $(SECRET_FILE) ]; then \
	  kubectl delete -n $(K8S_NAMESPACE) -f $(SECRET_FILE) || true; \
	fi
	@echo "✔️  All resources deleted."
