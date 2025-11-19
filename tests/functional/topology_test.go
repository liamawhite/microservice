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

func TestFaultInjection(t *testing.T) {
	ctx := context.Background()

	// Create a custom network for inter-container communication
	nw := createTestNetwork(t, ctx)

	// Define service configurations with explicit ports
	serviceConfigs := []ServiceConfig{
		{Name: "service-a", Port: "8080"},
		{Name: "service-b", Port: "8080"},
	}
	services := createServices(t, ctx, nw, serviceConfigs)

	// Test basic fault injection - always returns 500
	t.Run("fault_500_always", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%s/fault/500", services[0].Port)
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should contain fault injection message
		assert.Contains(t, string(body), "Fault injected")
		assert.Contains(t, string(body), "500")
		t.Logf("✓ Fault injection 500 successful")
	})

	// Test fault injection with 404
	t.Run("fault_404_always", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%s/fault/404", services[0].Port)
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Contains(t, string(body), "Fault injected")
		assert.Contains(t, string(body), "404")
		t.Logf("✓ Fault injection 404 successful")
	})

	// Test fault injection with 0% chance - should never trigger
	t.Run("fault_0_percent_never_triggers", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%s/fault/500/0", services[0].Port)

		// Try multiple times to ensure it never triggers
		for range 10 {
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			// Should always return 200 since fault never triggers
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// Should NOT contain fault injection message
			assert.NotContains(t, string(body), "Fault injected")
		}
		t.Logf("✓ Fault injection 0%% never triggered (10 requests)")
	})

	// Test fault injection with percentage - should trigger some of the time
	t.Run("fault_50_percent_triggers_sometimes", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%s/fault/503/50", services[0].Port)

		faultCount := 0
		successCount := 0
		totalRequests := 2000 // 100x more requests for better statistical confidence

		for range totalRequests {
			resp, err := http.Get(url)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			switch resp.StatusCode {
			case http.StatusServiceUnavailable:
				faultCount++
			case http.StatusOK:
				successCount++
			}

			_, _ = io.ReadAll(resp.Body) // Drain body
		}

		// With 50% chance over 2000 requests, expect roughly 1000 faults (±10% for variance)
		faultPercentage := float64(faultCount) / float64(totalRequests) * 100
		assert.Greater(t, faultCount, 0, "Should have at least one fault")
		assert.Greater(t, successCount, 0, "Should have at least one success")
		assert.InDelta(t, 50.0, faultPercentage, 10.0, "Fault percentage should be close to 50%% (±10%%)")
		t.Logf("✓ Fault injection 50%% triggered %d/%d times (%.1f%%)", faultCount, totalRequests, faultPercentage)
	})

	// Test fault injection chained with proxy - fault doesn't trigger, continues to next service
	t.Run("fault_0_percent_chains_to_proxy", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%s/fault/500/0/proxy/%s:%s",
			services[0].Port, services[1].Name, serviceConfigs[1].Port)

		resp, err := http.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should contain response from service-b (the proxy target)
		assert.Contains(t, string(body), services[1].Name)
		assert.NotContains(t, string(body), "Fault injected")
		t.Logf("✓ Fault injection 0%% chained to proxy: %s -> %s", services[0].Name, services[1].Name)
	})

	// Test fault injection that triggers immediately in chain
	t.Run("fault_100_percent_terminates_chain", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%s/fault/502/100/proxy/%s:%s",
			services[0].Port, services[1].Name, serviceConfigs[1].Port)

		resp, err := http.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Should return 502 and never reach service-b
		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Contains(t, string(body), "Fault injected")
		assert.Contains(t, string(body), "502")
		// Should NOT contain service-b response
		assert.NotContains(t, string(body), services[1].Name)
		t.Logf("✓ Fault injection 100%% terminated chain before reaching %s", services[1].Name)
	})
}
