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

## Fault Injection

The service supports fault injection to simulate service failures and test retry logic in service meshes like Istio/Envoy.

### Fault Injection Path Format

Fault injection uses the `/fault/` prefix with the following formats:

- `/fault/<status-code>` - Always inject the specified error (100% chance)
- `/fault/<status-code>/<percentage>` - Inject error with specified probability
- `/fault/<status-code>/<percentage>/proxy/...` - Chain with proxy segments

**Status Codes**: Only error codes (400-599) are allowed.

### Fault Injection Behavior

When a fault injection segment is encountered:

1. **Random Check**: Generate a random number to determine if fault should trigger based on percentage
2. **If Fault Triggers**: Return error response immediately and terminate the request chain
3. **If Fault Doesn't Trigger**: Continue processing the remaining path segments

This behavior enables retry testing - some requests fail while others succeed on retry.

### Examples

#### Basic Fault Injection

```bash
# Always return 500 Internal Server Error
curl http://localhost:8080/fault/500

# Always return 404 Not Found
curl http://localhost:8080/fault/404
```

Response:
```json
{
  "status": 500,
  "service": "service-name",
  "message": "Fault injected: 500 Internal Server Error"
}
```

#### Percentage-Based Fault Injection

```bash
# Return 503 error 30% of the time, success 70% of the time
curl http://localhost:8080/fault/503/30

# Never trigger fault (0% chance) - always succeed
curl http://localhost:8080/fault/500/0

# Always trigger fault (100% chance)
curl http://localhost:8080/fault/502/100
```

#### Fault Injection with Proxy Chains

```bash
# Inject 500 error 50% of the time, otherwise forward to service-b
# This simulates intermittent service failures
curl http://localhost:8080/fault/500/50/proxy/service-b:8080

# Chain through multiple services with fault injection
# If fault triggers, request terminates before reaching service-b
curl http://localhost:8080/fault/502/30/proxy/service-b:8080/proxy/service-c:9080
```

### Use Cases

**Service Mesh Retry Testing**: Test Istio/Envoy retry policies by injecting faults with a percentage. Some requests fail and trigger retries, while others succeed.

```bash
# With Istio retry policy configured, this tests automatic retries
# 40% of requests fail initially but may succeed on retry
curl http://localhost:8080/fault/503/40/proxy/backend:8080
```

**Circuit Breaker Testing**: Inject high error rates to trigger circuit breakers.

```bash
# Inject 90% error rate to test circuit breaker thresholds
curl http://localhost:8080/fault/500/90/proxy/backend:8080
```

**Timeout Testing**: Combine fault injection with timeouts to test timeout handling.

```bash
# Service configured with 30s timeout
# Inject errors to test timeout behavior
curl http://localhost:8080/fault/504/100
```

### Implementation Details

- **Path Parsing**: `parsePath()` function in `internal/proxy/handler.go:53-150` handles fault injection paths
- **Random Generation**: Uses `math/rand.Intn(100)` to determine if fault triggers based on percentage
- **Response Handler**: `sendFaultResponse()` function in `internal/proxy/handler.go:252-278` generates fault responses
- **Execution Logic**: `ServeHTTP()` function in `internal/proxy/handler.go:175-222` contains fault injection logic

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

The chart is published to GitHub Container Registry and can be installed directly from the OCI registry:

#### Basic Installation
```bash
# Install latest version from OCI registry
helm install my-microservice oci://ghcr.io/liamawhite/microservice

# Install specific version
helm install my-microservice oci://ghcr.io/liamawhite/microservice --version <version>
```

#### Single Service Deployment
```bash
helm install my-microservice oci://ghcr.io/liamawhite/microservice \
  --set services[0].name=web \
  --set services[0].port=8080
```

#### Three-Tier Services Deployment
```bash
helm install my-topology oci://ghcr.io/liamawhite/microservice \
  --set services[0].name=frontend \
  --set services[0].port=8080 \
  --set services[1].name=backend \
  --set services[1].port=8081 \
  --set services[2].name=database \
  --set services[2].port=8082
```

#### Custom Configuration with Values File
```bash
# Create custom values file
cat > my-values.yaml << EOF
defaults:
  image:
    tag: latest
  resources:
    requests:
      memory: "64Mi"
      cpu: "250m"
    limits:
      memory: "128Mi"
      cpu: "500m"

services:
  - name: frontend
    port: 8080
  - name: backend
    port: 8081
    resources:
      requests:
        memory: "128Mi"
      limits:
        memory: "256Mi"
EOF

# Install with custom values
helm install my-deployment oci://ghcr.io/liamawhite/microservice -f my-values.yaml
```

#### Local Development
For local development and testing, you can still install from the local chart directory:
```bash
# Install from local chart (for development)
helm install my-microservice ./chart/ -f my-values.yaml
```

#### Example Values Files

**Single Service** (`values-single.yaml`):
- Single microservice instance
- Basic resource limits
- Standard configuration

**Three-Tier Services** (`values-three-tier.yaml`):
- Frontend: Entry point service (port 8080)
- Backend: Application logic service with enhanced resources (port 8081)
- Database: Data storage service with autoscaling and anti-affinity (port 8082)

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

With the three-tier deployment, you can test proxy chains:

```bash
# Chain through frontend -> backend -> database
kubectl port-forward service/my-topology-frontend 8080:8080
curl http://localhost:8080/proxy/my-topology-backend:8081/proxy/my-topology-database:8082

# Direct access to backend
kubectl port-forward service/my-topology-backend 8081:8081
curl http://localhost:8081/health
```

## Configuration

### Command-Line Interface

The service uses [Cobra](https://github.com/spf13/cobra) for CLI argument parsing. The `serve` command starts the HTTP server and accepts the following flags:

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--port` | `-p` | int | 8080 | HTTP server port |
| `--timeout` | `-t` | duration | 30s | Request timeout |
| `--service-name` | `-s` | string | "proxy" | Service identifier in responses |
| `--log-level` | `-l` | string | "info" | Log level (debug, info, warn, error) |
| `--log-format` | `-f` | string | "json" | Log output format (json, text) |
| `--log-headers` | | bool | false | Log all request and response headers with sensitive data redaction |

### Usage Examples

```bash
# Using long flags
./microservice serve --port 8080 --service-name my-service --log-level debug

# Using short flags (recommended for brevity)
./microservice serve -p 8080 -s my-service -l debug

# Get help
./microservice serve --help

# Show version
./microservice --version
```

### Input Validation

The CLI validates all inputs before starting the server:
- **Port**: Must be between 1-65535
- **Timeout**: Must be positive (or zero for no timeout)
- **Log Level**: Must be one of: debug, info, warn, error
- **Log Format**: Must be one of: json, text

Invalid inputs will display a helpful error message and exit with code 1.

### Health Check
All services expose `/health` endpoint returning:
```json
{"status":"healthy","service":"service-name"}
```

