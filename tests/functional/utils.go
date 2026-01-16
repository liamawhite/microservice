package functional

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ServiceConfig represents the configuration for a single service
type ServiceConfig struct {
	Name string
	Port string
}

// ServiceResult represents a created service with its container and mapped port
type ServiceResult struct {
	Name      string
	Port      string
	Container testcontainers.Container
}

// createTestNetwork creates a Docker network for inter-container communication
// Returns the network with cleanup registered
func createTestNetwork(t *testing.T, ctx context.Context) *testcontainers.DockerNetwork {
	nw, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := nw.Remove(ctx); err != nil {
			t.Logf("Failed to remove network: %v", err)
		}
	})
	return nw
}

// createServices creates multiple containerized services with specified configurations
// Returns service results with containers and their mapped ports with cleanup and readiness checks
func createServices(t *testing.T, ctx context.Context, nw *testcontainers.DockerNetwork, serviceConfigs []ServiceConfig) []ServiceResult {
	results := make([]ServiceResult, len(serviceConfigs))

	// Start all services in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect shared slice

	for idx, config := range serviceConfigs {
		wg.Add(1)
		go func(idx int, config ServiceConfig) {
			defer wg.Done()

			exposedPort := fmt.Sprintf("%s/tcp", config.Port)

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
				WaitingFor: wait.ForHTTP("/health").
					WithPort(nat.Port(exposedPort)).
					WithStartupTimeout(30 * time.Second),
				Cmd: []string{
					"serve",
					fmt.Sprintf("--port=%s", config.Port),
					fmt.Sprintf("--service-name=%s", config.Name),
					"--log-format=text",
				},
			}

			container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: containerReq,
				Started:          true,
			})
			require.NoError(t, err)

			mappedPort, err := container.MappedPort(ctx, nat.Port(exposedPort))
			require.NoError(t, err)

			// Thread-safe assignment to shared slice
			mu.Lock()
			results[idx] = ServiceResult{
				Name:      config.Name,
				Port:      mappedPort.Port(),
				Container: container,
			}
			mu.Unlock()

			// Cleanup with conditional log dumping
			t.Cleanup(func() {
				// Only dump logs if the test failed
				if t.Failed() {
					dumpContainerLogs(t, ctx, container, config.Name)
				}

				if err := container.Terminate(ctx); err != nil {
					t.Logf("Failed to terminate container: %v", err)
				}
			})
		}(idx, config)
	}

	// Wait for all services to be created
	wg.Wait()

	return results
}

// dumpContainerLogs retrieves and dumps all container logs to test output
func dumpContainerLogs(t *testing.T, ctx context.Context, container testcontainers.Container, serviceName string) {
	t.Logf("=== Container logs for %s ===", serviceName)

	// Get container logs
	logs, err := container.Logs(ctx)
	if err != nil {
		t.Logf("Failed to get logs for %s: %v", serviceName, err)
		return
	}
	defer func() { _ = logs.Close() }()

	// Read all logs
	logBytes, err := io.ReadAll(logs)
	if err != nil {
		t.Logf("Failed to read logs for %s: %v", serviceName, err)
		return
	}

	// Print logs to test output
	if len(logBytes) > 0 {
		t.Logf("Logs for %s:\n%s", serviceName, string(logBytes))
	} else {
		t.Logf("No logs found for %s", serviceName)
	}

	t.Logf("=== End logs for %s ===", serviceName)
}
