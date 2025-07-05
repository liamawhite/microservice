# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based microservice that creates composable mock microservice topologies. The service acts as an HTTP proxy that can chain requests through multiple services to simulate complex microservice architectures for testing purposes.

## Development Environment

This project uses Nix for development environment management. Always run commands within the Nix development shell:

```bash
nix develop --command bash
```

The Makefile automatically uses the Nix shell via `SHELL := nix develop --command bash`.

## Core Architecture

### Proxy Chain System
- **Entry Point**: `cmd/main.go` - HTTP server with configurable ports, timeouts, and logging
- **Proxy Handler**: `internal/proxy/handler.go` - Core proxy logic that parses paths and forwards requests
- **Path Format**: `/proxy/service:port/proxy/next-service:port/...` - Chain multiple services together
- **Final Hop**: When no more `/proxy/` segments exist, the service returns its own response

### Request Flow
1. Request arrives at service with path like `/proxy/service-b:8080/proxy/service-c:80`
2. Handler parses path to extract next hop (`service-b:8080`) and remaining path (`/proxy/service-c:80`)
3. If remaining path exists, forwards request to next service
4. If no remaining path, returns final JSON response with service name

### Response Format
```json
{
  "status": 200,
  "service": "service-name",
  "message": "Request processed successfully"
}
```

## Common Commands

### Development
- `make check` - Format, lint, and tidy (run before committing)
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
- `make security` - Run gosec security scanner with medium severity and high confidence

### Docker
- `make docker-build` - Build multi-platform Docker image
- `make docker-build DOCKER_PUSH=true` - Build and push to registry

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

## Helm Chart

The service includes a Helm chart for Kubernetes deployment located in the `chart/` directory. The chart supports multi-service deployments with configurable defaults and per-service overrides.

### Installation

The chart includes example values files for different deployment scenarios:

#### Single Service Deployment
```bash
helm install my-microservice ./chart/ -f values-single.yaml
```

#### Three Services Deployment
```bash
helm install my-topology ./chart/ -f values-three-services.yaml
```

#### Custom Configuration
```bash
# Create your own values file
cp chart/values-single.yaml my-values.yaml
# Edit my-values.yaml as needed
helm install my-deployment ./chart/ -f my-values.yaml
```

#### Example Values Files

**Single Service** (`values-single.yaml`):
- Single microservice instance
- Basic resource limits
- Standard configuration

**Three Services** (`values-three-services.yaml`):
- Service A: Entry point (port 8080)
- Service B: Middle service with enhanced resources (port 8081)
- Service C: Final service with autoscaling and anti-affinity (port 8082)

Both examples demonstrate the flexibility of the defaults/services structure.

### Multi-Service Configuration

The chart supports a `defaults` section for common configuration and a `services` array where each service can override defaults:

- **defaults**: Base configuration applied to all services
- **services**: Array of service definitions with per-service overrides
- **global**: Global settings that apply to all services

Each service in the array creates:
- A separate Deployment with unique name: `<release-name>-<service-name>`
- A separate Service for network access
- Optional HPA if autoscaling is enabled
- Service accounts (shared or per-service based on configuration)

### Testing Multi-Service Topologies

With the multi-service deployment, you can test proxy chains:

```bash
# Chain through service-a -> service-b -> service-c
kubectl port-forward service/my-topology-service-a 8080:8080
curl http://localhost:8080/proxy/my-topology-service-b:8081/proxy/my-topology-service-c:8082

# Direct access to service-b
kubectl port-forward service/my-topology-service-b 8081:8081
curl http://localhost:8081/health
```

## Configuration

### Service Configuration
- **Port**: `-port` flag (default: 8080)
- **Timeout**: `-timeout` flag (default: 30s)  
- **Service Name**: `-service-name` flag (default: "proxy")
- **Log Level**: `-log-level` flag (debug, info, warn, error)
- **Log Format**: `-log-format` flag (json, text)

### Health Check
All services expose `/health` endpoint returning:
```json
{"status":"healthy","service":"service-name"}
```

