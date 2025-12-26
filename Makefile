# Determine root directory
ROOT_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Gather all .go files for use in dependencies below
GO_FILES=$(shell find $(ROOT_DIR) -name '*.go')

# Gather list of expected binaries
BINARIES=$(shell cd $(ROOT_DIR)/cmd && ls -1 | grep -v ^common)

# Output directory for binaries
BIN_DIR=$(ROOT_DIR)/bin

# Extract Go module name from go.mod
GOMODULE=$(shell grep ^module $(ROOT_DIR)/go.mod | awk '{ print $$2 }')

# Set version strings based on git tag and current ref
GO_LDFLAGS=-ldflags "-s -w -X '$(GOMODULE)/internal/cli.Version=$(shell git describe --tags --exact-match 2>/dev/null || echo dev)' -X '$(GOMODULE)/internal/cli.Commit=$(shell git rev-parse --short HEAD)' -X '$(GOMODULE)/internal/cli.BuildDate=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')'"

.PHONY: build build-bundled ffmpeg-assets mod-tidy clean test gen-docs

# Alias for building program binary
build: $(BINARIES)

build-bundled: ffmpeg-assets
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build \
		-tags ffmpeg_embedded \
		$(GO_LDFLAGS) \
		-o $(BIN_DIR)/lipi \
		./cmd/lipi

ffmpeg-assets:
	./scripts/fetch-ffmpeg-assets.sh

mod-tidy:
	# Needed to fetch new dependencies and add them to go.mod
	go mod tidy

clean:
	rm -rf $(BIN_DIR)

format: golines
	@go fmt ./...
	@gofmt -s -w $(GO_FILES)

golines:
	golines -w --ignore-generated --chain-split-dots --max-len=80 --reformat-tags .

lint:
	@echo "Running golangci-lint..."
	@golangci-lint run

lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	@golangci-lint run --fix

test: mod-tidy
	go test -v -race ./...

# Build our program binaries
# Depends on GO_FILES to determine when rebuild is needed
$(BINARIES): mod-tidy $(GO_FILES)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build \
		$(GO_LDFLAGS) \
		-o $(BIN_DIR)/$(@) \
		./cmd/$(@)
