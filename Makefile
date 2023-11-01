# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

BIN := contour-admission-webhook
VERSION := $(shell git describe --exact-match 2>/dev/null || basename $$(git describe --all --long 2>/dev/null))

GO_BUILD_LDFLAGS := \
	-s \
	-w

# Image URL to use all building/pushing image targets
IMG ?= $(BIN):$(VERSION)

all: build

test: check

.PHONY: check
check: fmt vet lint
	go test ./... -coverprofile cover.out

.PHONY: build
build: fmt vet lint
	go build -mod=readonly -ldflags "$(GO_BUILD_LDFLAGS)" -a -o bin/$(BIN) cmd/main.go

.PHONY: fmt
fmt:
	go fmt -mod=readonly ./...

.PHONY: vet
vet:
	go vet -mod=readonly -ldflags "$(GO_BUILD_LDFLAGS)" ./...

.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.1 run -v --exclude-use-default=false

.PHONY: docker-build
docker-build:
	docker build -t ${IMG} -f Dockerfile .

.PHONY: docker-push
docker-push:
	docker push ${IMG}

.PHONY: clean
clean:
	@rm -rf cover.out
	@rm -rf bin
