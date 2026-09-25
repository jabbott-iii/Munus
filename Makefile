SHELL := /bin/bash

# Usage:
#   make check                     Run every local quality check
#   make release VERSION=v0.1.0
#   make tag VERSION=v0.1.0
#   make push-tag VERSION=v0.1.0

GO ?= go
GOLANGCI_LINT ?= golangci-lint

# The SQLite driver (mattn/go-sqlite3) needs cgo and a C compiler.
export CGO_ENABLED := 1

.PHONY: help build test race cover vet fmt fmt-check lint check check-version tag push-tag release

help:
	@echo "Development targets:"
	@echo "  make build                    Build ./munus (needs a C compiler)"
	@echo "  make test                     Run unit tests"
	@echo "  make race                     Run unit tests with the race detector"
	@echo "  make cover                    Run tests and print total coverage (coverage.out)"
	@echo "  make vet                      Run go vet"
	@echo "  make fmt                      Format all Go files (gofmt -s -w .)"
	@echo "  make fmt-check                Fail if any Go file needs formatting"
	@echo "  make lint                     Run golangci-lint (CI uses v2.13.2); skipped if not installed"
	@echo "  make check                    fmt-check, vet, test, race and lint"
	@echo ""
	@echo "Release targets:"
	@echo "  make tag VERSION=vX.Y.Z       Create annotated git tag"
	@echo "  make push-tag VERSION=vX.Y.Z  Push tag to origin"
	@echo "  make release VERSION=vX.Y.Z   Create and push tag (triggers CD)"

build:
	$(GO) build -o munus .

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -n 1

vet:
	$(GO) vet ./...

fmt:
	gofmt -s -w .

fmt-check:
	@unformatted="$$(gofmt -s -l .)"; \
	if [[ -n "$$unformatted" ]]; then \
		echo "These files need gofmt -s -w:"; echo "$$unformatted"; exit 1; \
	fi

lint:
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		$(GOLANGCI_LINT) run; \
	else \
		echo "golangci-lint not installed; skipping (CI runs golangci-lint v2.13.2)"; \
	fi

check: fmt-check vet test race lint

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
