# microservice

A Go-based HTTP proxy service that creates composable mock microservice topologies for testing complex distributed systems.

## What it does

This service acts as an HTTP proxy that can chain requests through multiple services, allowing you to simulate complex microservice architectures. Each service can forward requests to other services in the chain, and inject configurable faults to test resilience and retry logic. Perfect for testing distributed system behaviors in development and CI/CD pipelines.

## Usage

### Running the service

```bash
# Using short flags (recommended)
microservice serve -p 8080 -s my-service

# Using long flags
microservice serve --port 8080 --service-name my-service

# During development
go run . serve -p 8080 -s my-service
```

### Creating proxy chains

Use the `/proxy/` path format to chain requests through multiple services:

```bash
# Chain through service-b:8080, then service-c:80
curl http://localhost:8080/proxy/service-b:8080/proxy/service-c:80

# Direct proxy to a single service
curl http://localhost:8080/proxy/service-b:8080
```

### HTTPS Support

Each hop in the proxy chain can specify HTTP or HTTPS:

```bash
# Proxy to HTTPS service
curl http://localhost:8080/proxy/https://service-b:8443

# Mixed protocol chain (HTTP -> HTTPS -> HTTP)
curl http://localhost:8080/proxy/https://service-a:8443/proxy/http://service-b:8080

# Run HTTPS server
microservice serve --tls-cert=cert.pem --tls-key=key.pem

# HTTPS server with self-signed cert (skip upstream TLS verification)
microservice serve --tls-cert=cert.pem --tls-key=key.pem --upstream-tls-insecure

# Test HTTPS chain
curl -k https://localhost:8443/proxy/https://service-b:9443
```

**Protocol syntax:**
- Default: HTTP if no protocol specified (`/proxy/service:8080`)
- Explicit HTTPS: `/proxy/https://service:8443`
- Explicit HTTP: `/proxy/http://service:8080`

### Fault injection

Simulate service failures and test retry logic using the `/fault/` path format:

```bash
# Always return 500 Internal Server Error
curl http://localhost:8080/fault/500

# Return 503 error 30% of the time (for testing retries)
curl http://localhost:8080/fault/503/30

# Inject faults in a proxy chain
# 50% chance of 500 error, otherwise forward to service-b
curl http://localhost:8080/fault/500/50/proxy/service-b:8080
```

**Path formats:**
- `/fault/<status-code>` - Always inject error (100% chance)
- `/fault/<status-code>/<percentage>` - Inject error with specified probability (0-100)
- `/fault/<status-code>/<percentage>/proxy/...` - Chain with proxy segments

**Supported status codes:** 400-599 (client and server errors)

**Use cases:**
- **Retry testing**: Test Istio/Envoy retry policies with percentage-based faults
- **Circuit breaker testing**: Inject high error rates to trigger circuit breakers
- **Resilience testing**: Validate application behavior under intermittent failures

**Example response:**
```json
{
  "status": 500,
  "service": "service-name",
  "message": "Fault injected: 500 Internal Server Error"
}
```

### How it works

**Proxy chains:**
1. Parse the path to extract the next service (`service-b:8080`)
2. Forward the request with the remaining path (`/proxy/service-c:80`)
3. Return the final response when no more proxy segments exist

**Fault injection:**
1. Parse the path to extract status code and percentage
2. Generate random number to determine if fault should trigger
3. If triggered: return error response immediately
4. If not triggered: continue to next segment or return success

### Health check

```bash
curl http://localhost:8080/health
```

## Configuration

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | `-p` | 8080 | HTTP/HTTPS server port |
| `--timeout` | `-t` | 30s | Request timeout |
| `--service-name` | `-s` | proxy | Service identifier in responses |
| `--log-level` | `-l` | info | Log level (debug, info, warn, error) |
| `--log-format` | `-f` | json | Log format (json, text) |
| `--log-headers` | | false | Log request/response headers with sensitive data redaction |
| `--tls-cert` | | "" | Path to TLS certificate (enables HTTPS with --tls-key) |
| `--tls-key` | | "" | Path to TLS key file (enables HTTPS with --tls-cert) |
| `--upstream-tls-insecure` | | false | Skip TLS verification for upstream HTTPS requests |

### CLI Help and Version

```bash
# Display all available commands and flags
microservice --help

# Get help for the serve command
microservice serve --help

# Show version information
microservice --version
# or
microservice version
```

### Input Validation

The CLI validates all inputs before starting the server:
- Port must be between 1 and 65535
- Timeout must be positive
- Log level must be one of: debug, info, warn, error
- Log format must be one of: json, text

Invalid inputs will display a helpful error message.

## Response Format

All services return JSON responses:

```json
{
  "status": 200,
  "service": "service-name",
  "message": "Request processed successfully"
}
```

Health endpoint response:
```json
{
  "status": "healthy",
  "service": "service-name"
}
```

## Docker

```bash
docker run -p 8080:8080 ghcr.io/liamawhite/microservice:latest
```

## Kubernetes Deployment

This project includes a Helm chart for deploying multi-service topologies on Kubernetes.

### Quick Start

Deploy a single service:
```bash
helm install my-microservice ./chart/ -f chart/values-single.yaml
```

Deploy a three-tier topology:
```bash
helm install my-topology ./chart/ -f chart/values-three-tier.yaml
```

### Features

- **Multi-Service Deployments**: Deploy interconnected microservice topologies
- **Flexible Configuration**: Global defaults with per-service overrides
- **Auto-scaling**: Optional HPA configuration per service
- **Service Discovery**: Native Kubernetes service-to-service communication
- **Example Configurations**: Pre-built values files for common scenarios

### Testing Proxy Chains on Kubernetes

Once deployed, test request chains between services:

```bash
# Port-forward to the entry service
kubectl port-forward service/my-topology-service-a 8080:8080

# Chain through multiple services
curl http://localhost:8080/proxy/my-topology-service-b:8081/proxy/my-topology-service-c:8082

# Test fault injection (50% error rate)
curl http://localhost:8080/fault/503/50/proxy/my-topology-service-b:8081

# Check individual service health
kubectl port-forward service/my-topology-service-b 8081:8081
curl http://localhost:8081/health
```

### Chart Documentation

See [chart/README.md](chart/README.md) for detailed Helm chart documentation, including:
- Configuration parameters
- Custom topology examples
- Resource naming conventions
- Installation options

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for development setup, testing, and contribution guidelines.
