SHELL := /bin/bash
GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
ifeq ($(strip $(VERSION)),)
VERSION := dev
endif
LDFLAGS := -s -w -X hookgram/internal/version.Version=$(VERSION)

build:
	@mkdir -p dist
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/hookgram ./cmd/server

test:
	@if [ -d web/node_modules ]; then printf 'module hookgram_node_modules\n\ngo 1.26\n' > web/node_modules/go.mod; fi
	$(GO) test ./...

vet:
	@if [ -d web/node_modules ]; then printf 'module hookgram_node_modules\n\ngo 1.26\n' > web/node_modules/go.mod; fi
	$(GO) vet ./...

release:
	bash ./scripts/release.sh