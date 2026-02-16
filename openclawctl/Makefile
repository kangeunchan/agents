SHELL := /bin/sh

BINARY := openclawctl
CMD_DIR := ./cmd/openclawctl
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MIN_COVERAGE ?= 25.0
GOBIN ?= $(shell go env GOBIN)
ifeq ($(strip $(GOBIN)),)
GOBIN := $(shell go env GOPATH)/bin
endif
INSTALL_BIN_DIR ?= $(HOME)/.local/bin
STATICCHECK := $(GOBIN)/staticcheck
LDFLAGS := -s -w \
	-X github.com/kangeunchan/openclawctl/internal/version.Version=$(VERSION) \
	-X github.com/kangeunchan/openclawctl/internal/version.Commit=$(COMMIT) \
	-X github.com/kangeunchan/openclawctl/internal/version.Date=$(DATE)

.PHONY: tidy fmt vet staticcheck lint test test-race coverage coverage-check build install ci buildx-check buildx-builder-check

tidy:
	go mod tidy

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

staticcheck:
	@test -x "$(STATICCHECK)" || GOBIN="$(GOBIN)" go install honnef.co/go/tools/cmd/staticcheck@latest
	"$(STATICCHECK)" ./...

lint: vet staticcheck

test:
	go test ./...

test-race:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage-check: coverage
	@TOTAL=$$(go tool cover -func=coverage.out | awk '/^total:/ {print substr($$3, 1, length($$3)-1)}'); \
	awk -v total=$$TOTAL -v min=$(MIN_COVERAGE) 'BEGIN { if (total+0 < min+0) { printf("coverage %.1f%% is below %.1f%%\n", total, min); exit 1 } else { printf("coverage %.1f%% >= %.1f%%\n", total, min) } }'

build:
	go build -trimpath -ldflags='$(LDFLAGS)' -o ./bin/$(BINARY) $(CMD_DIR)

install:
	mkdir -p "$(INSTALL_BIN_DIR)"
	GOBIN="$(INSTALL_BIN_DIR)" go install -trimpath -ldflags='$(LDFLAGS)' $(CMD_DIR)
	@echo "Installed $(BINARY) to $(INSTALL_BIN_DIR)/$(BINARY)"

ci: fmt lint test test-race coverage-check

buildx-builder-check:
	docker buildx build --platform linux/amd64,linux/arm64 --target openclawctl-builder --output=type=cacheonly -f ../openclaw/Dockerfile ..

buildx-check:
	docker buildx build --platform linux/amd64 --load -f ../openclaw/Dockerfile ..
