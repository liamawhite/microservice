package functional

import (
	"context"
	"io"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

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
