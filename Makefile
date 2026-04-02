VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

.PHONY: build test test-proof lint clean cross

build:
	go build $(LDFLAGS) -o bin/trvl ./cmd/trvl

test:
	go test ./...

test-proof:
	go test -v -count=1 -race ./...

lint:
	go vet ./...
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

clean:
	rm -f bin/trvl
	rm -rf dist/

cross:
	GOOS=linux  GOARCH=amd64 go build $(LDFLAGS) -o dist/trvl-linux-amd64  ./cmd/trvl
	GOOS=linux  GOARCH=arm64 go build $(LDFLAGS) -o dist/trvl-linux-arm64  ./cmd/trvl
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/trvl-darwin-amd64 ./cmd/trvl
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/trvl-darwin-arm64 ./cmd/trvl
