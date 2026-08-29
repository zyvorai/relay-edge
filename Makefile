# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0

.PHONY: test vet build smoke smoke-all release-binaries

GO ?= go
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='$(LDFLAGS)' -o bin/relay-edge ./cmd/relay-edge

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
