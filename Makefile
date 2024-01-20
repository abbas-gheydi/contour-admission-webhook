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

## Tool Binaries
ENVTEST ?= $(LOCALBIN)/setup-envtest

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.28.0

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

all: build

test: check

.PHONY: check
check: fmt vet lint envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

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
