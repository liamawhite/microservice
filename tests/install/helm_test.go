package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelmTemplate(t *testing.T) {
	tests := []struct {
		name       string
		valuesFile string
		goldenFile string
	}{
		{
			name:       "default",
			valuesFile: "chart/values.yaml",
			goldenFile: "testdata/golden/default.yaml",
		},
		{
			name:       "single-service",
			valuesFile: "chart/values-single.yaml",
			goldenFile: "testdata/golden/single-service.yaml",
		},
		{
			name:       "three-tier",
			valuesFile: "chart/values-three-tier.yaml",
			goldenFile: "testdata/golden/three-tier.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the project root directory (two levels up from tests/install)
			projectRoot, err := filepath.Abs("../..")
			if err != nil {
				t.Fatalf("Failed to get project root: %v", err)
			}

			// Run helm template command
			cmd := exec.Command("helm", "template", "test-release", "./chart/", "-f", tt.valuesFile)
			cmd.Dir = projectRoot
			output, err := cmd.Output()
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					t.Fatalf("helm template failed: %v\nStderr: %s", err, exitError.Stderr)
				}
				t.Fatalf("helm template failed: %v", err)
			}

			actualOutput := strings.TrimSpace(string(output))

			// Read golden file
			goldenPath := filepath.Join(projectRoot, "tests/install", tt.goldenFile)
			expectedOutput, err := os.ReadFile(goldenPath)
			if err != nil {
				// If UPDATE_GOLDEN environment variable is set, create/update the golden file
				if os.Getenv("UPDATE_GOLDEN") == "true" {
					if err := os.WriteFile(goldenPath, []byte(actualOutput+"\n"), 0644); err != nil {
						t.Fatalf("Failed to update golden file: %v", err)
					}
					t.Logf("Updated golden file: %s", goldenPath)
					return
				}
				t.Fatalf("Failed to read golden file %s: %v", goldenPath, err)
			}

			expectedOutputStr := strings.TrimSpace(string(expectedOutput))

			// Compare outputs
			if actualOutput != expectedOutputStr {
				// If UPDATE_GOLDEN environment variable is set, update the golden file
				if os.Getenv("UPDATE_GOLDEN") == "true" {
					if err := os.WriteFile(goldenPath, []byte(actualOutput+"\n"), 0644); err != nil {
						t.Fatalf("Failed to update golden file: %v", err)
					}
					t.Logf("Updated golden file: %s", goldenPath)
					return
				}

				t.Errorf("helm template output differs from golden file.\n\nExpected:\n%s\n\nActual:\n%s", expectedOutputStr, actualOutput)
			}
		})
	}
}
