package functional

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestProxyChainWithMultipleServices(t *testing.T) {
	ctx := context.Background()

	// Create a custom network for inter-container communication
	nw, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := nw.Remove(ctx); err != nil {
			t.Logf("Failed to remove network: %v", err)
		}
	})

	// Start multiple proxy service instances
	services := make([]testcontainers.Container, 3)
	servicePorts := make([]string, 3)
	serviceNames := []string{"service-a", "service-b", "service-c"}

	// Start all services in parallel
	for idx, serviceName := range serviceNames {
		containerReq := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    "../..",
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"8080/tcp"},
			Networks:     []string{nw.Name},
			NetworkAliases: map[string][]string{
				nw.Name: {serviceName},
			},
			WaitingFor: wait.ForHTTP("/health").
				WithPort("8080/tcp").
				WithStartupTimeout(30 * time.Second),
			Cmd: []string{
				"-port=8080",
				fmt.Sprintf("-service-name=%s", serviceName),
				"-log-format=text",
			},
		}

		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: containerReq,
			Started:          true,
		})
		require.NoError(t, err)

		port, err := container.MappedPort(ctx, "8080")
		require.NoError(t, err)

		services[idx] = container
		servicePorts[idx] = port.Port()

		// Cleanup with conditional log dumping
		t.Cleanup(func() {
			// Only dump logs if the test failed
			if t.Failed() {
				dumpContainerLogs(t, ctx, container, serviceName)
			}

			if err := container.Terminate(ctx); err != nil {
				t.Logf("Failed to terminate container: %v", err)
			}
		})
	}

	// Wait for all services to be ready
	time.Sleep(2 * time.Second)

	// Test 1: Direct health checks to verify all services are running
	t.Run("health_checks", func(t *testing.T) {
		for i, serviceName := range serviceNames {
			url := fmt.Sprintf("http://localhost:%s/health", servicePorts[i])
			resp, err := http.Get(url)
			require.NoError(t, err, "Health check failed for %s", serviceName)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "Service %s is not healthy", serviceName)
			t.Logf("✓ %s is healthy on port %s", serviceName, servicePorts[i])
		}
	})

	// Test 2: Single proxy request (service-a)
	t.Run("single_proxy", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%s/", servicePorts[0])
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		if !assert.Equal(t, http.StatusOK, resp.StatusCode) {
			// If the test fails, dump additional debug info
			t.Logf("Single proxy test failed. URL: %s, Status: %d", url, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", string(body))
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should contain the response from service-a
		assert.Contains(t, string(body), serviceNames[0])
		t.Logf("✓ Single proxy successful: %s", serviceNames[0])
	})

	// Test 3: Chain proxy request (service-a -> service-b -> service-c)
	t.Run("chain_proxy", func(t *testing.T) {
		// Use container names for internal communication
		url := fmt.Sprintf("http://localhost:%s/proxy/%s:8080/proxy/%s:8080",
			servicePorts[0], serviceNames[1], serviceNames[2])

		resp, err := http.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			// If the test fails, dump additional debug info
			t.Logf("Chain proxy test failed. URL: %s, Status: %d", url, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", string(body))

			// Dump logs from all containers for debugging
			for i, container := range services {
				t.Logf("=== DEBUG LOGS for %s ===", serviceNames[i])
				dumpContainerLogs(t, ctx, container, serviceNames[i])
			}
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should contain the response from the final service (service-c)
		assert.Contains(t, string(body), serviceNames[2])
		t.Logf("✓ Chain proxy successful: %s -> %s -> %s", serviceNames[0], serviceNames[1], serviceNames[2])
	})

}
