.PHONY: build test lint fmt vet clean install release ci

BINARY  := orro
MAIN    := ./cmd/orro
VERSION := 0.2.0
LDFLAGS := -ldflags "-X github.com/yashiels/orro-cli/internal/config.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) $(MAIN)

install:
	go install $(LDFLAGS) $(MAIN)

test:
	go test ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; skipping (install: brew install golangci-lint)"; \
	fi

fmt:
	gofmt -w .

vet:
	go vet ./...

check: vet test

ci: lint test

clean:
	rm -f $(BINARY)
	rm -rf dist/

# Cross-compile all release targets.
release: clean
	mkdir -p dist
	GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64  $(MAIN)
	GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64  $(MAIN)
	GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64   $(MAIN)
	GOOS=linux   GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64   $(MAIN)
	GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe $(MAIN)