# Contributing to plutigo

Thank you for your interest in contributing to plutigo! This document provides project-specific guidelines for contributors to plutigo.

For general Blink Labs contributing guidelines (including conventional commits, DCO, licensing, and general PR processes), please see the [organization-wide contributing guide](https://github.com/blinklabs-io/.github/blob/main/CONTRIBUTING.md).

## Development Setup

### Prerequisites

- Go 1.24 or later
- make
- git

### Getting Started

1. Fork the repository on GitHub
2. Clone your fork:
   ```sh
   git clone https://github.com/your-username/plutigo.git
   cd plutigo
   ```
3. Set up the development environment:
   ```sh
   go mod tidy
   ```
4. Verify everything works:
   ```sh
   make test
   ```

## Development Workflow

### 1. Choose an Issue

- Check the [GitHub Issues](https://github.com/blinklabs-io/plutigo/issues) for open tasks
- For beginners, look for issues labeled `good first issue`

### 2. Create a Branch

Create a feature branch from `main`:

```sh
git checkout -b feature/your-feature-name
```

### 3. Make Changes

- Follow Go best practices and idioms
- Add tests for new functionality
- Update documentation as needed
- Ensure code passes all checks

### 4. Testing

Run the full test suite:

```sh
# Unit tests
make test

# Benchmarks (check for regressions)
make bench

# Fuzz tests
make fuzz

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 5. Code Quality

Ensure your code meets our standards:

```sh
# Format code
make format

# Lint (must pass with zero issues)
golangci-lint run

# Check for nil pointer issues
nilaway ./...
```

### 6. Commit

Write clear, descriptive commit messages following [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```sh
git add .
git commit -s -m "feat: add new builtin function support

- Implement xyz builtin
- Add comprehensive tests
- Update documentation"
```

### 7. Submit Pull Request

- Push your branch to GitHub
- Create a pull request against the `main` branch
- Provide a clear description of your changes
- Link to any relevant issues

## Code Guidelines

### Go Standards

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `go fmt` for formatting
- Follow standard Go naming conventions
- Keep functions focused and small

### Testing

- Write tests for all new functionality
- Aim for high test coverage (currently 52%)
- Use table-driven tests where appropriate
- Add benchmarks for performance-critical code

### Documentation

- Update README.md for user-facing changes
- Add code comments for complex algorithms

### Performance

- Profile before optimizing
- Add benchmarks for performance changes
- Ensure no performance regressions

## Areas for Contribution

### High Priority

- Improving test coverage (currently 52%)
- Performance optimizations
- Documentation improvements
- Bug fixes

### Medium Priority

- Additional built-in function implementations
- Better error messages
- Code refactoring for maintainability

### Low Priority

- Alternative backends (WASM, etc.)
- Integration with other Cardano tools
- Advanced debugging features

## Getting Help

- Open an issue for questions
- Join our [Discord](https://discord.gg/5fPRZnX4qW) for discussions
- Check existing issues/PRs for similar work

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (Apache 2.0).