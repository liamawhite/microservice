package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateFlags(t *testing.T) {
	tests := []struct {
		name        string
		setupFlags  func()
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid flags with defaults",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "json"
			},
			expectError: false,
		},
		{
			name: "valid port at minimum",
			setupFlags: func() {
				port = 1
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "json"
			},
			expectError: false,
		},
		{
			name: "valid port at maximum",
			setupFlags: func() {
				port = 65535
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "json"
			},
			expectError: false,
		},
		{
			name: "invalid port - too low",
			setupFlags: func() {
				port = 0
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "json"
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			setupFlags: func() {
				port = 65536
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "json"
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "invalid port - negative",
			setupFlags: func() {
				port = -1
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "json"
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "invalid timeout - negative",
			setupFlags: func() {
				port = 8080
				timeout = -5 * time.Second
				logLevel = "info"
				logFormat = "json"
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
		},
		{
			name: "valid timeout - zero is allowed",
			setupFlags: func() {
				port = 8080
				timeout = 0
				logLevel = "info"
				logFormat = "json"
			},
			expectError: false,
		},
		{
			name: "valid log level - debug",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "debug"
				logFormat = "json"
			},
			expectError: false,
		},
		{
			name: "valid log level - warn",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "warn"
				logFormat = "json"
			},
			expectError: false,
		},
		{
			name: "valid log level - error",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "error"
				logFormat = "json"
			},
			expectError: false,
		},
		{
			name: "invalid log level",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "invalid"
				logFormat = "json"
			},
			expectError: true,
			errorMsg:    "log-level must be one of [debug, info, warn, error]",
		},
		{
			name: "invalid log level - case sensitive",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "INFO"
				logFormat = "json"
			},
			expectError: true,
			errorMsg:    "log-level must be one of [debug, info, warn, error]",
		},
		{
			name: "valid log format - text",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "text"
			},
			expectError: false,
		},
		{
			name: "invalid log format",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "xml"
			},
			expectError: true,
			errorMsg:    "log-format must be one of [json, text]",
		},
		{
			name: "invalid log format - case sensitive",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "JSON"
			},
			expectError: true,
			errorMsg:    "log-format must be one of [json, text]",
		},
		{
			name: "cert provided without key file",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "json"
				tlsCertFile = "/path/to/cert.pem"
				tlsKeyFile = ""
			},
			expectError: true,
			errorMsg:    "both --tls-cert and --tls-key must be provided together",
		},
		{
			name: "key provided without cert file",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "json"
				tlsCertFile = ""
				tlsKeyFile = "/path/to/key.pem"
			},
			expectError: true,
			errorMsg:    "both --tls-cert and --tls-key must be provided together",
		},
		{
			name: "cert and key with non-existent files",
			setupFlags: func() {
				port = 8080
				timeout = 30 * time.Second
				logLevel = "info"
				logFormat = "json"
				tlsCertFile = "/nonexistent/cert.pem"
				tlsKeyFile = "/nonexistent/key.pem"
			},
			expectError: true,
			errorMsg:    "certificate file not found",
		},
		{
			name: "all valid options combined",
			setupFlags: func() {
				port = 9090
				timeout = 60 * time.Second
				serviceName = "test-service"
				logLevel = "debug"
				logFormat = "text"
				logHeaders = true
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags to defaults before each test
			port = 8080
			timeout = 30 * time.Second
			serviceName = "proxy"
			logLevel = "info"
			logFormat = "json"
			logHeaders = false
			tlsCertFile = ""
			tlsKeyFile = ""
			upstreamTLSInsecure = false

			// Setup test-specific flags
			tt.setupFlags()

			// Run validation
			err := validateFlags(nil, nil)

			// Check results
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectError && err != nil && tt.errorMsg != "" {
				// Check if error message contains expected substring
				if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// generateTestCertificates creates a self-signed certificate for testing
func generateTestCertificates(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	// Create temporary directory
	tmpDir := t.TempDir()

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Write certificate to file
	certPath = filepath.Join(tmpDir, "cert.pem")
	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("failed to create cert file: %v", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("failed to encode certificate: %v", err)
	}

	// Write private key to file
	keyPath = filepath.Join(tmpDir, "key.pem")
	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	defer keyFile.Close()

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes}); err != nil {
		t.Fatalf("failed to encode private key: %v", err)
	}

	return certPath, keyPath
}

func TestValidateFlagsWithTLS(t *testing.T) {
	// Generate test certificates
	certPath, keyPath := generateTestCertificates(t)

	t.Run("valid tls configuration", func(t *testing.T) {
		// Reset flags to defaults
		port = 8080
		timeout = 30 * time.Second
		serviceName = "proxy"
		logLevel = "info"
		logFormat = "json"
		logHeaders = false
		tlsCertFile = certPath
		tlsKeyFile = keyPath
		upstreamTLSInsecure = false

		// Run validation
		err := validateFlags(nil, nil)

		// Should not error with valid cert and key
		if err != nil {
			t.Errorf("unexpected error with valid TLS config: %v", err)
		}
	})

	t.Run("valid tls with insecure flag", func(t *testing.T) {
		// Reset flags to defaults
		port = 8080
		timeout = 30 * time.Second
		serviceName = "proxy"
		logLevel = "info"
		logFormat = "json"
		logHeaders = false
		tlsCertFile = certPath
		tlsKeyFile = keyPath
		upstreamTLSInsecure = true

		// Run validation
		err := validateFlags(nil, nil)

		// Should not error
		if err != nil {
			t.Errorf("unexpected error with valid TLS config and insecure flag: %v", err)
		}
	})
}
