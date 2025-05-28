# Makefile for Go project
.PHONY: test test-match bench fmt clean play play-fmt play-flat build download-plutus-tests

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

test-match: ## Run specific tests (usage: make test-one TEST=TestName)
	@echo "Running test: $(TEST)..."
	@go test -run $(TEST) -v ./...

bench: ## Run tests
	@echo "Running benchmarks..."
	@go test -v -bench=. -run='^$$' ./...

fmt: ## Format Go code
	@gofmt -w -s .

clean: ## Remove test cache
	@go clean -testcache

build: ## Build play
	@go build -o play ./cmd/play/

play: ## Run some uplc sample code
	@go run ./cmd/play/ cmd/play/sample.uplc

play-flat: ## Run some uplc from flat
	@go run ./cmd/play/ cmd/play/auction_1-1.flat

play-fmt: ## Format the uplc sample code
	@go run ./cmd/play/ -f cmd/play/sample.uplc

download-plutus-tests:
	@echo "Downloading latest plutus tests..."

	@rm -rf tests/conformance

	@curl -L -s https://github.com/IntersectMBO/plutus/archive/master.tar.gz | tar xz -C /tmp

	@mkdir -p tests/conformance

	@mv /tmp/plutus-master/plutus-conformance/test-cases/uplc/evaluation/* tests/conformance/

	@rm -rf /tmp/plutus-master

	@echo "Download complete. Test cases are now in tests/conformance/"
