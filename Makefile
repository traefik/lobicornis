.PHONY: clean fmt check test build build-crossbinary

GOFILES := $(shell git ls-files '*.go' | grep -v '^vendor/')

TAG_NAME := $(shell git tag -l --contains HEAD)
SHA := $(shell git rev-parse --short HEAD)
VERSION := $(if $(TAG_NAME),$(TAG_NAME),$(SHA))
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%I:%M:%S%p')

VERSION_PACKAGE=github.com/containous/lobicornis/meta

default: clean check test build-crossbinary

test: clean
	go test -v -cover ./...

dependencies:
	dep ensure -v

clean:
	rm -rf dist/ cover.out

build: clean
	@echo Version: $(VERSION) $(BUILD_DATE)
	go build -v -ldflags '-X "${VERSION_PACKAGE}.Version=${VERSION}" -X "${VERSION_PACKAGE}.BuildDate=${BUILD_DATE}"'

check:
	golangci-lint run

fmt:
	@gofmt -s -l -w $(GOFILES)

build-crossbinary:
	./_script/crossbinary
