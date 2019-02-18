.PHONY: clean fmt check test build build-crossbinary

GOFILES := $(shell git ls-files '*.go' | grep -v '^vendor/')

default: clean check test build-crossbinary

test: clean
	go test -v -cover ./...

dependencies:
	dep ensure -v

clean:
	rm -rf dist/ cover.out

build:
	go build

check:
	golangci-lint run

fmt:
	@gofmt -s -l -w $(GOFILES)

build-crossbinary:
	./_script/crossbinary
