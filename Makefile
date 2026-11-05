BIN := bin/cockpit
GOBIN := $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test lint install init clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/cockpit

test:
	go test ./...

lint:
	go vet ./...
	test -z "$$(gofmt -l .)"

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/cockpit

init: install
	$(GOBIN)/cockpit init

clean:
	rm -rf bin
