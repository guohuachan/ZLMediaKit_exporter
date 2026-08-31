#
# Tools
#

GO = go
GOFMT = gofmt
GOCI = golangci-lint
ZLMEXPORTER_IMAGE = zlm_exporter
IMAGE_TAG = latest

#
# Version stamping
#

VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BRANCH     ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

VERSION_PKG = github.com/prometheus/common/version
LDFLAGS = -s -w \
	-X $(VERSION_PKG).Version=$(VERSION) \
	-X $(VERSION_PKG).Revision=$(COMMIT_SHA) \
	-X $(VERSION_PKG).Branch=$(BRANCH) \
	-X $(VERSION_PKG).BuildDate=$(BUILD_DATE) \
	-X main.BuildVersion=$(VERSION) \
	-X main.BuildCommitSha=$(COMMIT_SHA) \
	-X main.BuildDate=$(BUILD_DATE)

#
# Format
#

.PHONY: fmt lint lintfix test test_cover test_file build build-docker run

# check code style in these directories
FMT_DIRS = .

fmt:
	$(GOFMT) -d -s -w $(FMT_DIRS)

lint:
	$(GOCI) run --timeout=10m

lintfix:
	$(GOCI) run --fix


TEST_FLAGS := \
	-v -race -failfast -p=1 \
	-covermode=atomic \

test:
	$(GO) test -v ./... -failfast

test_cover:
	$(GO) test -v $(TEST_FLAGS) -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

test_file:
	$(GO) test -v $(FILE)

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o zlm_exporter .

build-docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT_SHA=$(COMMIT_SHA) \
		--build-arg BRANCH=$(BRANCH) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(ZLMEXPORTER_IMAGE):$(IMAGE_TAG) .

run:
	$(GO) run .
