# job-radar — developer tasks.
# Run `make help` for a list.

BINARY := job-radar
PKG := ./...

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help.
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the job-radar binary into ./bin.
	go build -o bin/$(BINARY) ./cmd/job-radar

.PHONY: run
run: ## Run job-radar (pass args with ARGS="run --dry-run").
	go run ./cmd/job-radar $(ARGS)

.PHONY: test
test: ## Run all tests with the race detector.
	go test -race $(PKG)

.PHONY: vet
vet: ## Run go vet.
	go vet $(PKG)

.PHONY: fmt
fmt: ## Format all Go code.
	gofmt -s -w .

.PHONY: tidy
tidy: ## Tidy go.mod/go.sum.
	go mod tidy

.PHONY: hooks
hooks: ## Install the git hooks (pre-commit secret/private-data guard).
	git config core.hooksPath scripts/git-hooks
	@echo "✓ git hooks installed (core.hooksPath=scripts/git-hooks)"

.PHONY: check
check: fmt vet test ## Format, vet and test — run before committing.
