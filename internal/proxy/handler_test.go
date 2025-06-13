package proxy

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    actions
		wantErr bool
	}{
		{
			name: "empty path",
			path: "",
			want: actions{
				NextHop:   "",
				Remaining: "/",
				IsLastHop: true,
			},
		},
		{
			name: "trailing slash",
			path: "/",
			want: actions{
				NextHop:   "",
				Remaining: "/",
				IsLastHop: true,
			},
		},
		{
			name:    "non-proxy prefix",
			path:    "/api/abcdef",
			want:    actions{},
			wantErr: true,
		},
		{
			name:    "missing service",
			path:    "/proxy",
			want:    actions{},
			wantErr: true,
		},
		{
			name:    "empty service name",
			path:    "/proxy//",
			want:    actions{},
			wantErr: true,
		},
		{
			name: "single service with default port",
			path: "/proxy/svca",
			want: actions{
				NextHop:   "svca",
				Remaining: "/",
				IsLastHop: false,
			},
			wantErr: false,
		},
		{
			name: "single service with custom port",
			path: "/proxy/svca:8080",
			want: actions{
				NextHop:   "svca:8080",
				Remaining: "/",
				IsLastHop: false,
			},
			wantErr: false,
		},
		{
			name: "two services with default ports",
			path: "/proxy/svca/proxy/svcb",
			want: actions{
				NextHop:   "svca",
				Remaining: "/proxy/svcb",
				IsLastHop: false,
			},
			wantErr: false,
		},
		{
			name: "two services with custom ports",
			path: "/proxy/svca:8080/proxy/svcb:9080",
			want: actions{
				NextHop:   "svca:8080",
				Remaining: "/proxy/svcb:9080",
				IsLastHop: false,
			},
			wantErr: false,
		},
		{
			name: "two services mixed ports",
			path: "/proxy/svca:8080/proxy/svcb",
			want: actions{
				NextHop:   "svca:8080",
				Remaining: "/proxy/svcb",
				IsLastHop: false,
			},
			wantErr: false,
		},
		{
			name: "three services with custom ports",
			path: "/proxy/svca:8080/proxy/svcb:9080/proxy/svcc:10080",
			want: actions{
				NextHop:   "svca:8080",
				Remaining: "/proxy/svcb:9080/proxy/svcc:10080",
				IsLastHop: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePath(tt.path)

			if tt.wantErr {
				require.Error(t, err, "parsePath() should return error")
				return
			}

			require.NoError(t, err, "parsePath() should not return error")
			assert.Equal(t, tt.want, got, "parsePath() result mismatch")
		})
	}
}

func TestNewHandler(t *testing.T) {
	logger := createTestLogger()
	timeout := 30 * time.Second
	serviceName := "test-service"

	handler := NewHandler(timeout, serviceName, logger)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.client)
	assert.Equal(t, timeout, handler.timeout)
	assert.Equal(t, serviceName, handler.serviceName)
	assert.Equal(t, logger, handler.logger)
	assert.Equal(t, timeout, handler.client.Timeout)
}

// createTestLogger creates a test logger that outputs to stderr for debugging
func createTestLogger() *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: false, // Don't add source for cleaner test output
	}
	handler := slog.NewTextHandler(os.Stderr, opts)
	return slog.New(handler).With(slog.String("test", "true"))
}
