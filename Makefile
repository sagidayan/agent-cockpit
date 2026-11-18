BIN := bin/cockpit
GOBIN := $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test e2e lint install init clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/cockpit

test:
	go test ./...

e2e:
	go test -tags e2e -count=1 ./e2e/

lint:
	go vet -tags e2e ./...
	test -z "$$(gofmt -l .)"

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/cockpit

init: install
	$(GOBIN)/cockpit init

clean:
	rm -rf bin
