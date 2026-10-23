GO_FILES := $(shell find cmd internal -type f -name '*.go')
APP := ./cmd/bot
DIST := dist

.PHONY: generate fmt fmt-check lint check build build-all build-linux build-windows build-macos

generate:
	sqlc generate

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "Files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

lint:
	golangci-lint run

check: fmt-check lint

build: build-all

build-all: build-linux build-windows build-macos

build-linux:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(DIST)/algobot-linux-amd64 $(APP)

build-windows:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(DIST)/algobot-windows-amd64.exe $(APP)

build-macos:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $(DIST)/algobot-darwin-amd64 $(APP)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $(DIST)/algobot-darwin-arm64 $(APP)
