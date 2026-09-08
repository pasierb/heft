BINARY := bin/heft
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install test run clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/heft

install: build
	install -Dm755 $(BINARY) $(HOME)/.local/bin/heft

test:
	go test ./...

run:
	go run ./cmd/heft

clean:
	rm -rf bin
