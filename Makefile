# Makefile for Go project
.PHONY: test test-one fmt clean play play-fmt

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

test-one: ## Run specific tests (usage: make test-one TEST=TestName)
	@echo "Running test: $(TEST)..."
	@go test -run $(TEST) -v ./...

fmt: ## Format Go code
	@gofmt -w -s .

clean: ## Remove test cache
	@go clean -testcache

play: ## Run the play command with arguments
	@go run ./cmd/play/ cmd/play/sample.uplc

play-fmt: ## Run the play command with arguments
	@go run ./cmd/play/ -f cmd/play/sample.uplc
