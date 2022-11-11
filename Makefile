GO_FLAGS += "-ldflags=-s -w"
GO_FLAGS += -trimpath
BINARY_NAME=csgove
BIN_PATH=bin

.DEFAULT_GOAL := help

build-unixlike:
	@test -n "$(GOOS)" || (echo "The environment variable GOOS must be provided" && false)
	@test -n "$(GOARCH)" || (echo "The environment variable GOARCH must be provided" && false)
	CGO_ENABLED=1 GOOS="$(GOOS)" GOARCH="$(GOARCH)" go build $(GO_FLAGS) -o "$(BIN_PATH)/$(BINARY_NAME)"

build-darwin: ## Build for Darwin amd64
	@$(MAKE) GOOS=darwin GOARCH=amd64 build-unixlike

build-linux: ## Build for Linux
	@$(MAKE) GOOS=linux GOARCH=amd64 build-unixlike

build-windows: ## Build for Windows
	CGO_ENABLED=1 GOOS=windows GOARCH=386 go build $(GO_FLAGS) -o $(BIN_PATH)/$(BINARY_NAME).exe

clean: ## Clean up project files
	rm -f *.wav *.bin && rm -rf bin

help:
	@echo 'Targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
