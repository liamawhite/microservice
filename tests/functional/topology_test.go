package functional

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyChainWithMultipleServices(t *testing.T) {
	ctx := context.Background()

	// Create a custom network for inter-container communication
	nw := createTestNetwork(t, ctx)

	// Define service configurations with explicit ports
	serviceConfigs := []ServiceConfig{
		{Name: "service-a", Port: "8080"},
		{Name: "service-b", Port: "8080"},
		{Name: "service-c", Port: "80"},
	}
	services := createServices(t, ctx, nw, serviceConfigs)

	// Single proxy request (service-a)
	t.Run("single_proxy", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%s/", services[0].Port)
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
		assert.Contains(t, string(body), services[0].Name)
		t.Logf("✓ Single proxy successful: %s", services[0].Name)
	})

	// Chain proxy request (service-a -> service-b -> service-c)
	t.Run("chain_proxy", func(t *testing.T) {
		// Use container names for internal communication with their respective ports
		url := fmt.Sprintf("http://localhost:%s/proxy/%s:%s/proxy/%s:%s",
			services[0].Port, services[1].Name, serviceConfigs[1].Port, services[2].Name, serviceConfigs[2].Port)

		resp, err := http.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			// If the test fails, dump additional debug info
			t.Logf("Chain proxy test failed. URL: %s, Status: %d", url, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", string(body))

			// Dump logs from all containers for debugging
			for _, service := range services {
				t.Logf("=== DEBUG LOGS for %s ===", service.Name)
				dumpContainerLogs(t, ctx, service.Container, service.Name)
			}
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should contain the response from the final service (service-c)
		assert.Contains(t, string(body), services[2].Name)
		t.Logf("✓ Chain proxy successful: %s -> %s -> %s", services[0].Name, services[1].Name, services[2].Name)
	})

}
