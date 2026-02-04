# Microservice Helm Chart

A Helm chart for deploying composable mock microservice topologies. This chart creates HTTP proxy services that can chain requests through multiple services to simulate complex microservice architectures for testing purposes.

## Features

- **Multi-Service Deployments**: Deploy multiple interconnected microservice instances
- **Flexible Configuration**: Global defaults with per-service overrides
- **Auto-scaling Support**: Optional HPA configuration per service
- **Service Discovery**: Kubernetes-native service discovery between components
- **Resource Management**: Configurable resource requests and limits per service
- **HTTPS/TLS Support**: Configure TLS certificates for secure communication

## Quick Start

### Single Service
```bash
helm install my-microservice ./chart/ -f values-single.yaml
```

### Three-Tier Topology
```bash
helm install my-topology ./chart/ -f values-three-tier.yaml
```

### HTTPS-Enabled Service
```bash
# Create TLS secret first
kubectl create secret tls my-tls-secret --cert=cert.pem --key=key.pem

# Install with HTTPS configuration
helm install my-https-service ./chart/ \
  --set services[0].name=secure-service \
  --set services[0].config.port=8443 \
  --set services[0].config.tls.existingSecret=my-tls-secret \
  --set services[0].config.tls.insecure=true

# Or use the HTTPS example values file
helm install my-https-service ./chart/ -f values-https.yaml
```

## Configuration

The chart uses a two-tier configuration structure:

1. **`defaults`**: Base configuration applied to all services
2. **`services`**: Array of service definitions with per-service overrides

### Example Configuration

```yaml
# Global chart settings
nameOverride: ""
fullnameOverride: ""
imagePullSecrets: []

# Default values for all services
defaults:
  replicaCount: 1
  image:
    repository: ghcr.io/liamawhite/microservice
    tag: "latest"
  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"
  config:
    timeout: "30s"
    logLevel: "info"

# Service definitions
services:
  - name: "service-a"
    config:
      serviceName: "service-a"
      port: 8080
    service:
      port: 8080
  
  - name: "service-b"
    config:
      serviceName: "service-b"
      port: 8081
    service:
      port: 8081
    resources:
      requests:
        memory: "256Mi"
        cpu: "200m"
```

## Configuration Parameters

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nameOverride` | Override chart name | `""` |
| `fullnameOverride` | Override full release name | `""` |
| `imagePullSecrets` | Image pull secrets | `[]` |

### Default Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `defaults.replicaCount` | Number of replicas | `1` |
| `defaults.image.repository` | Container image repository | `ghcr.io/liamawhite/microservice` |
| `defaults.image.tag` | Container image tag | `"latest"` |
| `defaults.image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `defaults.service.type` | Kubernetes service type | `ClusterIP` |
| `defaults.service.port` | Service port | `8080` |
| `defaults.config.serviceName` | Service identifier | `"microservice"` |
| `defaults.config.port` | Container port | `8080` |
| `defaults.config.timeout` | Request timeout | `"30s"` |
| `defaults.config.logLevel` | Log level | `"info"` |
| `defaults.config.logFormat` | Log format | `"json"` |
| `defaults.config.logHeaders` | Log request/response headers | `false` |
| `defaults.config.tls.existingSecret` | Existing TLS secret name | `""` |
| `defaults.config.tls.cert` | TLS certificate (base64) | `""` |
| `defaults.config.tls.key` | TLS private key (base64) | `""` |
| `defaults.config.tls.insecure` | Skip upstream TLS verification | `false` |

### Service-Specific Parameters

Each service in the `services` array can override any default parameter:

| Parameter | Description |
|-----------|-------------|
| `name` | Service name (required) |
| `config.*` | Override microservice configuration |
| `service.*` | Override Kubernetes service settings |
| `resources.*` | Override resource requests/limits |
| `autoscaling.*` | Override autoscaling configuration |

## Example Values Files

The chart includes pre-configured examples:

### Single Service (`values-single.yaml`)
- Single microservice instance
- Resource limits configured
- Health checks enabled

### Three-Tier Services (`values-three-tier.yaml`)
- **Frontend**: Entry point service (port 8080)
- **Backend**: Application logic service with enhanced resources (port 8081)
- **Database**: Data storage service with autoscaling (port 8082)

### HTTPS Service (`values-https.yaml`)
- Single service with HTTPS/TLS enabled
- Self-signed certificate support with insecure mode
- Examples for both existingSecret and inline certificates

## Usage Patterns

### Testing Proxy Chains

Once deployed, test request chains between services:

```bash
# Port-forward to service-a
kubectl port-forward service/my-topology-service-a 8080:8080

# Chain through service-a -> service-b -> service-c
curl http://localhost:8080/proxy/my-topology-service-b:8081/proxy/my-topology-service-c:8082

# Direct access to service-b
kubectl port-forward service/my-topology-service-b 8081:8081
curl http://localhost:8081/health
```

### Testing HTTPS Proxy Chains

For HTTPS-enabled services:

```bash
# Port-forward to HTTPS service
kubectl port-forward service/my-https-frontend 8443:8443

# HTTPS chain with explicit protocol
curl -k https://localhost:8443/proxy/https://my-https-backend:8444

# Mixed HTTP/HTTPS chain
curl -k https://localhost:8443/proxy/http://my-http-service:8080/proxy/https://my-https-backend:8444

# Health check over HTTPS
curl -k https://localhost:8443/health
```

Note: Use `-k` (insecure) flag with curl when testing with self-signed certificates.

### Custom Topologies

Create custom topologies by defining your own services array:

```yaml
services:
  - name: "frontend"
    config:
      serviceName: "frontend"
      port: 8080
  - name: "backend"
    config:
      serviceName: "backend"
      port: 8081
  - name: "database"
    config:
      serviceName: "database"
      port: 8082
```

### HTTPS Configuration

Configure HTTPS/TLS for secure communication between services.

#### Using Existing TLS Secret (Recommended)

```yaml
# Create TLS secret first
# kubectl create secret tls my-tls-secret --cert=cert.pem --key=key.pem

services:
  - name: "secure-service"
    config:
      serviceName: "secure-service"
      port: 8443
      tls:
        existingSecret: "my-tls-secret"
        insecure: false  # Set to true for self-signed certs
    service:
      port: 8443
    livenessProbe:
      httpGet:
        path: /health
        port: http
        scheme: HTTPS
    readinessProbe:
      httpGet:
        path: /health
        port: http
        scheme: HTTPS
```

#### Using Inline Certificates

```yaml
services:
  - name: "secure-service"
    config:
      serviceName: "secure-service"
      port: 8443
      tls:
        # Base64-encoded certificate and key
        cert: "LS0tLS1CRUdJTi..."
        key: "LS0tLS1CRUdJTi..."
        insecure: true  # Required for self-signed certs
    service:
      port: 8443
```

#### Generating Test Certificates

For testing purposes, generate self-signed certificates:

```bash
# Generate self-signed certificate
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout key.pem -out cert.pem -days 365 \
  -subj "/CN=localhost"

# Create Kubernetes secret
kubectl create secret tls test-tls-secret \
  --cert=cert.pem --key=key.pem

# Use in Helm chart
helm install my-service ./chart/ \
  --set services[0].config.tls.existingSecret=test-tls-secret \
  --set services[0].config.tls.insecure=true
```

#### Mixed HTTP/HTTPS Topologies

Deploy services with different protocols:

```yaml
services:
  # HTTP frontend
  - name: "frontend"
    config:
      serviceName: "frontend"
      port: 8080
    service:
      port: 8080

  # HTTPS backend
  - name: "backend"
    config:
      serviceName: "backend"
      port: 8443
      tls:
        existingSecret: "backend-tls"
        insecure: true
    service:
      port: 8443

# Test mixed chain:
# curl http://frontend:8080/proxy/https://backend:8443
```

## Resources Created

For each service, the chart creates:

- **Deployment**: Runs the microservice container
- **Service**: Exposes the deployment within the cluster
- **ServiceAccount**: (Optional) Service account for the pods
- **HorizontalPodAutoscaler**: (Optional) Auto-scaling configuration

## Service Naming

Resources are named using the pattern: `<release-name>-<service-name>`

Example: For release `my-app` with service `service-a`:
- Deployment: `my-app-service-a`
- Service: `my-app-service-a`
- ServiceAccount: `my-app-service-a`

## Requirements

- Kubernetes 1.19+
- Helm 3.2.0+

## Installation from Repository

```bash
# Add repository (if published)
helm repo add microservice https://example.com/charts

# Install chart
helm install my-release microservice/microservice

# Install with custom values
helm install my-release microservice/microservice -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall my-release
```

## Contributing

See the main [repository README](../README.md) for development and contribution guidelines.