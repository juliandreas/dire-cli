# Dire Development Guide

Go-based CLI that automatically processes missing translations in your i18n JSON files using translation and AI providers, glossary definitions, and translation memory

## Development Commands

```bash
# Initialize configuration file
go run cmd/dire/main.go init

# Run the CLI
go run cmd/dire/main.go

# Format code according to Go standards
go fmt ./...

# Run all tests
go test ./...

# Run tests with verbose output and coverage
go test -v -cover ./...

# Tidy dependencies
go mod tidy

# Check for outdated dependencies
go list -u -m all

# Update all dependencies to latest versions
go get -u ./...

# Update only patch versions (safer)
go get -u=patch ./...
```

## Linting

### Installing golangci-lint

Use the official binary installer (recommended)

### Running the linter

```bash
# Run linter
golangci-lint run

# Run linter with auto-fix
golangci-lint run --fix
```
