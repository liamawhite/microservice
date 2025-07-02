# Development Guide

## Prerequisites
- Nix (recommended) or Go 1.24+
- Docker (for functional tests)

## Development Environment

This project uses Nix for development environment management. Always run commands within the Nix development shell:

```bash
nix develop --command bash
```

The Makefile automatically uses the Nix shell via `SHELL := nix develop --command bash`.

## Commands

### Development
- `make check` - Format, lint, and tidy code (run before committing)
- `make test` - Run all tests
- `make test-coverage` - Run tests with coverage report and open HTML coverage
- `make fmt` - Format Go code
- `make lint` - Run golangci-lint
- `make tidy` - Tidy Go modules

### Testing
- `go test -v ./...` - Run all tests with verbose output
- `go test -v ./internal/proxy` - Run specific package tests
- `go test -v ./tests/functional` - Run functional tests (requires Docker)

### Security
- `make security` - Run gosec security scanner

### Docker
```bash
# Build multi-platform image
make docker-build

# Build and push to registry
make docker-build DOCKER_PUSH=true
```

## Testing Architecture

### Unit Tests
- Located in `internal/proxy/handler_test.go`
- Test individual proxy handler functions

### Functional Tests
- Located in `tests/functional/topology_test.go` 
- Use testcontainers to create real Docker networks with multiple services
- Test actual proxy chains with HTTP requests between containers
- Utilities in `tests/functional/utils.go` for container management

### Test Network Setup
Functional tests create isolated Docker networks where containers communicate using:
- Container names as hostnames (e.g., `service-a`, `service-b`)
- Explicit port configurations for each service
- Parallel container startup for performance

## Architecture

### Core Components
- **Entry Point**: `cmd/main.go` - HTTP server with configurable ports, timeouts, and logging
- **Proxy Handler**: `internal/proxy/handler.go` - Core proxy logic that parses paths and forwards requests

### Proxy Chain System
- **Path Format**: `/proxy/service:port/proxy/next-service:port/...` - Chain multiple services together
- **Final Hop**: When no more `/proxy/` segments exist, the service returns its own response

### Request Flow
1. Request arrives at service with path like `/proxy/service-b:8080/proxy/service-c:80`
2. Handler parses path to extract next hop (`service-b:8080`) and remaining path (`/proxy/service-c:80`)
3. If remaining path exists, forwards request to next service
4. If no remaining path, returns final JSON response with service name