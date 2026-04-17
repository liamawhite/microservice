package functional

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// generateCAAndSignedCerts creates a CA key pair and a server cert signed by that CA.
// dnsNames are included as SANs in the server cert so TLS hostname verification passes.
// Returns paths to the CA cert PEM, server cert PEM, and server key PEM.
func generateCAAndSignedCerts(t *testing.T, dnsNames []string) (caPath, certPath, keyPath string) {
	t.Helper()
	require.NotEmpty(t, dnsNames, "dnsNames must not be empty")
	tmpDir := t.TempDir()

	// Generate CA key pair
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	caPath = filepath.Join(tmpDir, "ca.pem")
	caFile, err := os.Create(caPath)
	require.NoError(t, err)
	err = pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	require.NoError(t, err)
	_ = caFile.Close()

	// Generate server key pair signed by CA
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: dnsNames[0]},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsNames,
	}

	serverDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, caCert, &serverKey.PublicKey, caKey)
	require.NoError(t, err)

	certPath = filepath.Join(tmpDir, "cert.pem")
	certFile, err := os.Create(certPath)
	require.NoError(t, err)
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	require.NoError(t, err)
	_ = certFile.Close()

	keyPath = filepath.Join(tmpDir, "key.pem")
	keyFile, err := os.Create(keyPath)
	require.NoError(t, err)
	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})
	require.NoError(t, err)
	_ = keyFile.Close()

	return caPath, certPath, keyPath
}

// generateTestCertificates creates a self-signed certificate for testing
func generateTestCertificates(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	// Create temporary directory
	tmpDir := t.TempDir()

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

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
	require.NoError(t, err)

	// Write certificate to file
	certPath = filepath.Join(tmpDir, "cert.pem")
	certFile, err := os.Create(certPath)
	require.NoError(t, err)
	defer func() { _ = certFile.Close() }()

	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, err)

	// Write private key to file
	keyPath = filepath.Join(tmpDir, "key.pem")
	keyFile, err := os.Create(keyPath)
	require.NoError(t, err)
	defer func() { _ = keyFile.Close() }()

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes})
	require.NoError(t, err)

	return certPath, keyPath
}

// createHTTPSClient creates an HTTP client that skips TLS verification for testing
func createHTTPSClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

// createHTTPSService creates a single containerized service with HTTPS enabled
func createHTTPSService(t *testing.T, ctx context.Context, nw *testcontainers.DockerNetwork, config ServiceConfig, certPath, keyPath string) ServiceResult {
	exposedPort := fmt.Sprintf("%s/tcp", config.Port)

	// Define paths for certificates in the container
	containerCertPath := "/tmp/cert.pem"
	containerKeyPath := "/tmp/key.pem"

	containerReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../..",
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{exposedPort},
		Networks:     []string{nw.Name},
		NetworkAliases: map[string][]string{
			nw.Name: {config.Name},
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      certPath,
				ContainerFilePath: containerCertPath,
				FileMode:          0644,
			},
			{
				HostFilePath:      keyPath,
				ContainerFilePath: containerKeyPath,
				FileMode:          0644,
			},
		},
		WaitingFor: wait.ForHTTP("/health").
			WithPort(nat.Port(exposedPort)).
			WithTLS(true, &tls.Config{InsecureSkipVerify: true}).
			WithStartupTimeout(30 * time.Second),
		Cmd: []string{
			"serve",
			fmt.Sprintf("--port=%s", config.Port),
			fmt.Sprintf("--service-name=%s", config.Name),
			"--log-format=text",
			fmt.Sprintf("--tls-cert=%s", containerCertPath),
			fmt.Sprintf("--tls-key=%s", containerKeyPath),
			"--upstream-tls-insecure",
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: containerReq,
		Started:          true,
	})
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, nat.Port(exposedPort))
	require.NoError(t, err)

	result := ServiceResult{
		Name:      config.Name,
		Port:      mappedPort.Port(),
		Container: container,
	}

	// Cleanup with conditional log dumping
	t.Cleanup(func() {
		if t.Failed() {
			dumpContainerLogs(t, ctx, container, config.Name)
		}

		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	})

	return result
}

func TestHTTPSProxyChain(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx := context.Background()
	certPath, keyPath := generateTestCertificates(t)

	nw := createTestNetwork(t, ctx)

	// Create HTTPS services
	serviceA := createHTTPSService(t, ctx, nw, ServiceConfig{
		Name: "service-a",
		Port: "8443",
		TLS:  true,
	}, certPath, keyPath)

	serviceB := createHTTPSService(t, ctx, nw, ServiceConfig{
		Name: "service-b",
		Port: "8443",
		TLS:  true,
	}, certPath, keyPath)
	// serviceB is needed for the proxy chain but not directly referenced
	_ = serviceB

	// Test HTTPS chain: service-a -> service-b
	t.Run("https_chain", func(t *testing.T) {
		client := createHTTPSClient()
		url := fmt.Sprintf("https://localhost:%s/proxy/https://service-b:8443",
			serviceA.Port)

		resp, err := client.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var response map[string]interface{}
		err = json.Unmarshal(body, &response)
		require.NoError(t, err)

		// Should return service-b as the final service
		assert.Equal(t, "service-b", response["service"])
		assert.Equal(t, float64(200), response["status"])
	})

	// Test direct HTTPS health check
	t.Run("https_health_check", func(t *testing.T) {
		client := createHTTPSClient()
		url := fmt.Sprintf("https://localhost:%s/health", serviceA.Port)

		resp, err := client.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var response map[string]interface{}
		err = json.Unmarshal(body, &response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response["status"])
		assert.Equal(t, "service-a", response["service"])
	})
}

func TestMixedHTTPAndHTTPS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx := context.Background()
	certPath, keyPath := generateTestCertificates(t)

	nw := createTestNetwork(t, ctx)

	// Create one HTTP service and one HTTPS service
	httpService := createServices(t, ctx, nw, []ServiceConfig{
		{Name: "http-service", Port: "8080", TLS: false},
	})[0]

	httpsService := createHTTPSService(t, ctx, nw, ServiceConfig{
		Name: "https-service",
		Port: "8443",
		TLS:  true,
	}, certPath, keyPath)

	// Test HTTP service forwarding to HTTPS service
	t.Run("http_to_https_with_explicit_scheme", func(t *testing.T) {
		// Create HTTP service with explicit HTTPS upstream scheme
		exposedPort := "8080/tcp"
		containerReq := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    "../..",
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{exposedPort},
			Networks:     []string{nw.Name},
			NetworkAliases: map[string][]string{
				nw.Name: {"http-to-https-bridge"},
			},
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      certPath,
					ContainerFilePath: "/tmp/cert.pem",
					FileMode:          0644,
				},
				{
					HostFilePath:      keyPath,
					ContainerFilePath: "/tmp/key.pem",
					FileMode:          0644,
				},
			},
			WaitingFor: wait.ForHTTP("/health").
				WithPort(nat.Port(exposedPort)).
				WithStartupTimeout(30 * time.Second),
			Cmd: []string{
				"serve",
				"--port=8080",
				"--service-name=http-to-https-bridge",
				"--log-format=text",
				"--upstream-tls-insecure",
			},
		}

		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: containerReq,
			Started:          true,
		})
		require.NoError(t, err)
		defer func() { _ = container.Terminate(ctx) }()

		mappedPort, err := container.MappedPort(ctx, nat.Port(exposedPort))
		require.NoError(t, err)

		// Test HTTP request that forwards to HTTPS service
		client := &http.Client{Timeout: 30 * time.Second}
		url := fmt.Sprintf("http://localhost:%s/proxy/https://https-service:8443",
			mappedPort.Port())

		resp, err := client.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var response map[string]interface{}
		err = json.Unmarshal(body, &response)
		require.NoError(t, err)

		// Should successfully reach the HTTPS service
		assert.Equal(t, "https-service", response["service"])
	})

	// Verify both services exist
	t.Run("verify_services", func(t *testing.T) {
		assert.NotNil(t, httpService.Container)
		assert.NotNil(t, httpsService.Container)
	})
}

func TestAutoSchemeDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx := context.Background()
	certPath, keyPath := generateTestCertificates(t)

	nw := createTestNetwork(t, ctx)

	// Read cert and key files
	containerCertPath := "/tmp/cert.pem"
	containerKeyPath := "/tmp/key.pem"

	// Create service with auto scheme detection
	exposedPort := "8443/tcp"
	containerReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../..",
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{exposedPort},
		Networks:     []string{nw.Name},
		NetworkAliases: map[string][]string{
			nw.Name: {"auto-service"},
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      certPath,
				ContainerFilePath: containerCertPath,
				FileMode:          0644,
			},
			{
				HostFilePath:      keyPath,
				ContainerFilePath: containerKeyPath,
				FileMode:          0644,
			},
		},
		WaitingFor: wait.ForHTTP("/health").
			WithPort(nat.Port(exposedPort)).
			WithTLS(true, &tls.Config{InsecureSkipVerify: true}).
			WithStartupTimeout(30 * time.Second),
		Cmd: []string{
			"serve",
			"--port=8443",
			"--service-name=auto-service",
			"--log-format=text",
			fmt.Sprintf("--tls-cert=%s", containerCertPath),
			fmt.Sprintf("--tls-key=%s", containerKeyPath),
			"--upstream-tls-insecure",
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: containerReq,
		Started:          true,
	})
	require.NoError(t, err)
	defer func() { _ = container.Terminate(ctx) }()

	mappedPort, err := container.MappedPort(ctx, nat.Port(exposedPort))
	require.NoError(t, err)

	t.Run("auto_detects_https", func(t *testing.T) {
		client := createHTTPSClient()
		url := fmt.Sprintf("https://localhost:%s/health", mappedPort.Port())

		resp, err := client.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var response map[string]interface{}
		err = json.Unmarshal(body, &response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response["status"])
		assert.Equal(t, "auto-service", response["service"])
	})
}

func TestCustomCABundle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")
	}

	ctx := context.Background()
	nw := createTestNetwork(t, ctx)

	// Generate a CA and a server cert signed by it, with the upstream's DNS alias as a SAN.
	caPath, certPath, keyPath := generateCAAndSignedCerts(t, []string{"upstream-service"})

	containerCertPath := "/tmp/cert.pem"
	containerKeyPath := "/tmp/key.pem"
	containerCAPath := "/tmp/ca.pem"
	upstreamPort := "8443"
	upstreamExposedPort := upstreamPort + "/tcp"

	// Start the upstream HTTPS service using the CA-signed cert.
	upstreamReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../..",
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{upstreamExposedPort},
		Networks:     []string{nw.Name},
		NetworkAliases: map[string][]string{
			nw.Name: {"upstream-service"},
		},
		Files: []testcontainers.ContainerFile{
			{HostFilePath: certPath, ContainerFilePath: containerCertPath, FileMode: 0644},
			{HostFilePath: keyPath, ContainerFilePath: containerKeyPath, FileMode: 0644},
		},
		WaitingFor: wait.ForHTTP("/health").
			WithPort(nat.Port(upstreamExposedPort)).
			WithTLS(true, &tls.Config{InsecureSkipVerify: true}).
			WithStartupTimeout(30 * time.Second),
		Cmd: []string{
			"serve",
			fmt.Sprintf("--port=%s", upstreamPort),
			"--service-name=upstream-service",
			"--log-format=text",
			fmt.Sprintf("--tls-cert=%s", containerCertPath),
			fmt.Sprintf("--tls-key=%s", containerKeyPath),
		},
	}

	upstream, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: upstreamReq,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if t.Failed() {
			dumpContainerLogs(t, ctx, upstream, "upstream-service")
		}
		_ = upstream.Terminate(ctx)
	})

	// Sub-test: proxy with --additional-ca-cert trusts the upstream's CA.
	t.Run("with_custom_ca", func(t *testing.T) {
		proxyExposedPort := "8080/tcp"
		proxyReq := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    "../..",
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{proxyExposedPort},
			Networks:     []string{nw.Name},
			NetworkAliases: map[string][]string{
				nw.Name: {"proxy-with-ca"},
			},
			Files: []testcontainers.ContainerFile{
				{HostFilePath: caPath, ContainerFilePath: containerCAPath, FileMode: 0644},
			},
			WaitingFor: wait.ForHTTP("/health").
				WithPort(nat.Port(proxyExposedPort)).
				WithStartupTimeout(30 * time.Second),
			Cmd: []string{
				"serve",
				"--port=8080",
				"--service-name=proxy-with-ca",
				"--log-format=text",
				fmt.Sprintf("--additional-ca-cert=%s", containerCAPath),
			},
		}

		proxy, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: proxyReq,
			Started:          true,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			if t.Failed() {
				dumpContainerLogs(t, ctx, proxy, "proxy-with-ca")
			}
			_ = proxy.Terminate(ctx)
		})

		mappedPort, err := proxy.MappedPort(ctx, nat.Port(proxyExposedPort))
		require.NoError(t, err)

		client := &http.Client{Timeout: 30 * time.Second}
		url := fmt.Sprintf("http://localhost:%s/proxy/https://upstream-service:%s", mappedPort.Port(), upstreamPort)

		resp, err := client.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var response map[string]any
		err = json.Unmarshal(body, &response)
		require.NoError(t, err)

		assert.Equal(t, "upstream-service", response["service"])
		assert.Equal(t, float64(200), response["status"])
	})

	// Sub-test: proxy without --additional-ca-cert (and without --upstream-tls-insecure)
	// cannot verify the upstream's self-signed CA and returns 502.
	t.Run("without_custom_ca", func(t *testing.T) {
		proxyExposedPort := "8080/tcp"
		proxyReq := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    "../..",
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{proxyExposedPort},
			Networks:     []string{nw.Name},
			NetworkAliases: map[string][]string{
				nw.Name: {"proxy-without-ca"},
			},
			WaitingFor: wait.ForHTTP("/health").
				WithPort(nat.Port(proxyExposedPort)).
				WithStartupTimeout(30 * time.Second),
			Cmd: []string{
				"serve",
				"--port=8080",
				"--service-name=proxy-without-ca",
				"--log-format=text",
			},
		}

		proxy, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: proxyReq,
			Started:          true,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			if t.Failed() {
				dumpContainerLogs(t, ctx, proxy, "proxy-without-ca")
			}
			_ = proxy.Terminate(ctx)
		})

		mappedPort, err := proxy.MappedPort(ctx, nat.Port(proxyExposedPort))
		require.NoError(t, err)

		client := &http.Client{Timeout: 30 * time.Second}
		url := fmt.Sprintf("http://localhost:%s/proxy/https://upstream-service:%s", mappedPort.Port(), upstreamPort)

		resp, err := client.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Without trusting the CA, TLS verification fails and we expect a 502 Bad Gateway.
		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	})
}
