BINARY := ndtui
BIN_DIR := bin
GO ?= go

GOOS_CURRENT := $(shell $(GO) env GOOS)
EXT :=
ifeq ($(GOOS_CURRENT),windows)
EXT := .exe
endif
TARGET := $(BIN_DIR)/$(BINARY)$(EXT)

.PHONY: all build run test lint fmt tidy clean install windows linux darwin

all: test build

build:
	$(GO) build -trimpath -ldflags="-s -w" -o $(TARGET) ./cmd/$(BINARY)

run:
	$(GO) run ./cmd/$(BINARY)

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

install:
	$(GO) install -trimpath ./cmd/$(BINARY)

# Cross-compilation targets use shell-style environment prefixes and therefore
# require a Unix shell (Git Bash / MSYS / WSL on Windows).
windows:
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -o $(BIN_DIR)/$(BINARY)-windows-amd64.exe ./cmd/$(BINARY)

linux:
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath -o $(BIN_DIR)/$(BINARY)-linux-amd64 ./cmd/$(BINARY)

darwin:
	GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -o $(BIN_DIR)/$(BINARY)-darwin-arm64 ./cmd/$(BINARY)

clean:
	$(RM) -r $(BIN_DIR)
