package cmd

import (
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
