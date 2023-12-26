GO_FLAGS += "-ldflags=-s -w"
GO_FLAGS += -trimpath
GO_FLAGS += -tags nolibopusfile
BINARY_NAME=csgove

.DEFAULT_GOAL := help

build-unixlike:
	@test -n "$(GOOS)" || (echo "The environment variable GOOS must be provided" && false)
	@test -n "$(GOARCH)" || (echo "The environment variable GOARCH must be provided" && false)
	@test -n "$(BIN_DIR)" || (echo "The environment variable BIN_DIR must be provided" && false)
	CGO_ENABLED=1 GOOS="$(GOOS)" GOARCH="$(GOARCH)" go build $(GO_FLAGS) -o "$(BIN_DIR)/$(BINARY_NAME)"

build-darwin: ## Build for Darwin
	@test -f dist/bin/darwin-x64/libopus.0.dylib || (echo "dist/bin/darwin-x64/libopus.0.dylib is missing" && false)
	@$(MAKE) GOOS=darwin GOARCH=amd64 CGO_LDFLAGS="-L/usr/local/Cellar" BIN_DIR=dist/bin/darwin-x64 build-unixlike

build-linux: ## Build for Linux
	@test -f dist/bin/linux-x64/libopus.so.0 || (echo "dist/bin/linux-x64/libopus.so.0 is missing" && false)
	@$(MAKE) GOOS=linux GOARCH=amd64 BIN_DIR=dist/bin/linux-x64 build-unixlike

build-windows: ## Build for Windows
	@test -f dist/bin/win32-x64/opus.dll || (echo "dist/bin/win32-x64/opus.dll is missing" && false)
	PKG_CONFIG_PATH=$(shell realpath .) CGO_ENABLED=1 GOOS=windows GOARCH=386 go build $(GO_FLAGS) -o dist/bin/win32-x64/$(BINARY_NAME).exe

clean: ## Clean up project files
	rm -f *.wav *.bin

help:
	@echo 'Targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
