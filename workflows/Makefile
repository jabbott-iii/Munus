SHELL := /bin/bash

# Usage:
#   make release VERSION=v0.1.0
#   make tag VERSION=v0.1.0
#   make push-tag VERSION=v0.1.0

.PHONY: help check-version tag push-tag release

help:
	@echo "Targets:"
	@echo "  make tag VERSION=vX.Y.Z       Create annotated git tag"
	@echo "  make push-tag VERSION=vX.Y.Z  Push tag to origin"
	@echo "  make release VERSION=vX.Y.Z   Create and push tag (triggers CD)"

check-version:
	@if [[ -z "$(VERSION)" ]]; then \
		echo "ERROR: VERSION is required (example: VERSION=v0.1.0)"; \
		exit 1; \
	fi
	@if [[ ! "$(VERSION)" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z\.-]+)?$$ ]]; then \
		echo "ERROR: VERSION must look like vMAJOR.MINOR.PATCH (example: v1.2.3)"; \
		exit 1; \
	fi

tag: check-version
	@git rev-parse --is-inside-work-tree >/dev/null
	@if git rev-parse "$(VERSION)" >/dev/null 2>&1; then \
		echo "ERROR: tag $(VERSION) already exists locally"; \
		exit 1; \
	fi
	git tag -a "$(VERSION)" -m "Release $(VERSION)"
	@echo "Created tag $(VERSION)"

push-tag: check-version
	@git rev-parse --is-inside-work-tree >/dev/null
	@if ! git rev-parse "$(VERSION)" >/dev/null 2>&1; then \
		echo "ERROR: tag $(VERSION) does not exist locally. Run: make tag VERSION=$(VERSION)"; \
		exit 1; \
	fi
	git push origin "$(VERSION)"
	@echo "Pushed tag $(VERSION)"

release: tag push-tag
	@echo "Release tag $(VERSION) created and pushed."
