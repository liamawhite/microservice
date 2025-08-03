# microservice

A Go-based HTTP proxy service that creates composable mock microservice topologies for testing complex distributed systems.

## What it does

This service acts as an HTTP proxy that can chain requests through multiple services, allowing you to simulate complex microservice architectures. Each service can forward requests to other services in the chain, making it perfect for testing distributed system behaviors.

## Usage

### Running the service

```bash
go run cmd/main.go -port 8080 -service-name my-service
```

### Creating proxy chains

Use the `/proxy/` path format to chain requests through multiple services:

```bash
# Chain through service-b:8080, then service-c:80
curl http://localhost:8080/proxy/service-b:8080/proxy/service-c:80

# Direct proxy to a single service
curl http://localhost:8080/proxy/service-b:8080
```

### How it works

1. Parse the path to extract the next service (`service-b:8080`)
2. Forward the request with the remaining path (`/proxy/service-c:80`)
3. Return the final response when no more proxy segments exist

### Health check

```bash
curl http://localhost:8080/health
```

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | 8080 | HTTP server port |
| `-timeout` | 30s | Request timeout |
| `-service-name` | proxy | Service identifier in responses |
| `-log-level` | info | Log level (debug, info, warn, error) |
| `-log-format` | text | Log format (json, text) |

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
