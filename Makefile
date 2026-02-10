# Determine root directory
ROOT_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Gather all .go files for use in dependencies below
GO_FILES=$(shell find $(ROOT_DIR) -name '*.go')

# Gather list of expected binaries
BINARIES=$(shell cd $(ROOT_DIR)/cmd && ls -1 | grep -v ^common)

# Common flags for fuzz tests
FUZZ_FLAGS ?= -run=^$ -fuzztime=10s

.PHONY: mod-tidy test test-match test-cover bench bench-baseline bench-compare fuzz format golines clean download-plutus-tests validate validate-quick validate-fix

mod-tidy:
	# Needed to fetch new dependencies and add them to go.mod
	@go mod tidy

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race ./...

test-cover: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-match: ## Run specific tests (usage: make test-one TEST=TestName)
	@echo "Running test: $(TEST)..."
	@go test -run $(TEST) -v ./...

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchtime=5s -run='^$$' ./...

bench-baseline: ## Run benchmarks and save as baseline
	@echo "Running benchmarks and saving baseline..."
	@go test -bench=. -benchtime=5s -run='^$$' ./... > bench-baseline.txt

bench-compare: ## Run benchmarks and compare against baseline
	@echo "Running benchmarks and comparing against baseline..."
	@if [ ! -f bench-baseline.txt ]; then \
		echo "No baseline found. Run 'make bench-baseline' first."; \
		exit 1; \
	fi
	@go test -bench=. -benchtime=5s -run='^$$' ./... > bench-current.txt
	@echo "Benchmark comparison:"
	@benchstat bench-baseline.txt bench-current.txt || echo "Install benchstat: go install golang.org/x/perf/cmd/benchstat@latest"

fuzz: ## Run fuzz tests
	@echo "Running fuzz tests..."
	@go test $(FUZZ_FLAGS) -fuzz=FuzzFromByte ./builtin
	@go test $(FUZZ_FLAGS) -fuzz=FuzzDecodeCBOR ./data
	@go test $(FUZZ_FLAGS) -fuzz=FuzzParse ./syn
	@go test $(FUZZ_FLAGS) -fuzz=FuzzPretty ./syn
	@go test $(FUZZ_FLAGS) -fuzz=FuzzLexerNextToken ./syn/lex
	@go test $(FUZZ_FLAGS) -fuzz=FuzzMachineRun ./cek

format: ## Format Go code
	@go fmt ./...
	@gofmt -s -w $(GO_FILES)

golines:
	@golines -w --ignore-generated --chain-split-dots --max-len=80 --reformat-tags .

clean: ## Remove test cache
	@go clean -testcache
	@rm -f $(BINARIES)

download-plutus-tests:
	@echo "Downloading latest plutus tests..."

	@rm -rf tests/conformance

	@curl -L -s https://github.com/IntersectMBO/plutus/archive/master.tar.gz | tar xz -C /tmp

	@mkdir -p tests/conformance

	@mv /tmp/plutus-master/plutus-conformance/test-cases/uplc/evaluation/* tests/conformance/

	@rm -rf /tmp/plutus-master

	@echo "Download complete. Test cases are now in tests/conformance/"

validate: ## Run all pre-commit validation checks
	@./scripts/validate.sh

validate-quick: ## Run quick pre-commit validation (skip benchmarks)
	@./scripts/validate.sh --quick

validate-fix: ## Run validation and auto-fix formatting issues
	@./scripts/validate.sh --fix
