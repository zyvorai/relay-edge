# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
.PHONY: test vet fmt build smoke smoke-all release-binaries qualify help ci

GO ?= go
VERSION ?= 0.1.2
LDFLAGS := -s -w -X main.version=$(VERSION)

test: ## Unit tests
	$(GO) test ./...

vet: ## go vet
	$(GO) vet ./...

fmt: ## Fail if any Go file needs gofmt
	@test -z "$$(gofmt -l .)" || (echo "Run gofmt on:"; gofmt -l .; exit 1)

build: ## Build relay-edge
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='$(LDFLAGS)' -o bin/relay-edge ./cmd/relay-edge

ci: fmt vet test build ## Local gate: gofmt, vet, tests, build

help: ## Show targets
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | sort | awk -F':.*## ' '{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

qualify: build
	python3 scripts/qualify-matrix.py

smoke: build
	@echo "Start relay-edge first, then: EDGE=http://127.0.0.1:18086 make smoke-all"

smoke-all:
	./scripts/smoke.sh
	./scripts/smoke-firewater.sh
	./scripts/smoke-remote-edge.sh
	./scripts/smoke-fleet.sh

release-binaries:
	mkdir -p dist
	@for pair in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
	  GOOS=$${pair%/*} GOARCH=$${pair#*/}; \
	  CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH $(GO) build -trimpath -ldflags='$(LDFLAGS)' \
	    -o dist/relay-edge-$$GOOS-$$GOARCH ./cmd/relay-edge; \
	done
	(cd dist && sha256sum relay-edge-* > SHA256SUMS)
