.PHONY: all

default: clean test-unit validate build

dependencies:
	dep ensure

build:
	go build

validate:
	./_script/make.sh validate-gofmt validate-govet validate-golint validate-misspell

test-unit:
	./_script/make.sh test-unit

clean:
	rm -f cover.out lobicornis
