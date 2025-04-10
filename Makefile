# Makefile for Go project
.PHONY: test fmt clean

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

fmt: ## Format Go code
	@gofmt -w -s .

clean: ## Remove test cache
	@go clean -testcache

