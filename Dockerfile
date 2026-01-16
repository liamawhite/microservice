# Build stage
FROM golang:1.24.4-alpine AS builder

# Install git and SSL certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod ./
COPY go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o app .

# Final stage
FROM scratch

# Copy SSL certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /build/app /app

# Set the entrypoint
ENTRYPOINT ["/app"] 