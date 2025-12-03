# Determine root directory
ROOT_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Gather all .go files for use in dependencies below
GO_FILES=$(shell find $(ROOT_DIR) -name '*.go')

# Gather list of expected binaries
BINARIES=$(shell cd $(ROOT_DIR)/cmd && ls -1 | grep -v ^common)

# Common flags for fuzz tests
FUZZ_FLAGS ?= -run=^$ -fuzztime=10s

.PHONY: mod-tidy test test-match bench fuzz format clean play play-fmt play-flat build download-plutus-tests

mod-tidy:
	# Needed to fetch new dependencies and add them to go.mod
	@go mod tidy

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race ./...

test-match: ## Run specific tests (usage: make test-one TEST=TestName)
	@echo "Running test: $(TEST)..."
	@go test -run $(TEST) -v ./...

bench: ## Run tests
	@echo "Running benchmarks..."
	@go test -v -bench=. -run='^$$' ./...

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
