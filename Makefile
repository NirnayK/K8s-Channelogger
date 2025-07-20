# ─── Main Makefile ──────────────────────────────────────────────────────────────

# Include build and deployment makefiles
include build.mk
include deploy.mk

.DEFAULT_GOAL := help

.PHONY: help

# ─── Help ──────────────────────────────────────────────────────────────────────

## Show this help message.
help: ## Show available Makefile targets with descriptions
	@echo "\nUsage: make [target] [ENV=test|production]\n"
	@echo "Environment Variables:"
	@echo "  ENV=production        - Use deploy/secret.yaml"
	@echo "  ENV=test (default)    - Use deploy/testenv/secret_test.yaml"
	@echo ""
	@echo "Available targets:"
	@grep -Eh '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-30s %s\n", $$1, $$2}'
	@echo ""
