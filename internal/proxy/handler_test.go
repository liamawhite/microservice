package proxy

import (
	"log/slog"
	"net/http"
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
		// Fault injection test cases
		{
			name: "fault injection basic - 500",
			path: "/fault/500",
			want: actions{
				NextHop:         "",
				Remaining:       "/",
				IsLastHop:       false,
				IsFault:         true,
				FaultCode:       500,
				FaultPercentage: 100,
			},
			wantErr: false,
		},
		{
			name: "fault injection basic - 404",
			path: "/fault/404",
			want: actions{
				NextHop:         "",
				Remaining:       "/",
				IsLastHop:       false,
				IsFault:         true,
				FaultCode:       404,
				FaultPercentage: 100,
			},
			wantErr: false,
		},
		{
			name: "fault injection with percentage",
			path: "/fault/500/30",
			want: actions{
				NextHop:         "",
				Remaining:       "/",
				IsLastHop:       false,
				IsFault:         true,
				FaultCode:       500,
				FaultPercentage: 30,
			},
			wantErr: false,
		},
		{
			name: "fault injection with 0 percentage",
			path: "/fault/503/0",
			want: actions{
				NextHop:         "",
				Remaining:       "/",
				IsLastHop:       false,
				IsFault:         true,
				FaultCode:       503,
				FaultPercentage: 0,
			},
			wantErr: false,
		},
		{
			name: "fault injection chained with proxy",
			path: "/fault/500/30/proxy/service-b:8080",
			want: actions{
				NextHop:         "",
				Remaining:       "/proxy/service-b:8080",
				IsLastHop:       false,
				IsFault:         true,
				FaultCode:       500,
				FaultPercentage: 30,
			},
			wantErr: false,
		},
		{
			name: "fault injection chained with multiple proxies",
			path: "/fault/502/50/proxy/service-a:8080/proxy/service-b:9080",
			want: actions{
				NextHop:         "",
				Remaining:       "/proxy/service-a:8080/proxy/service-b:9080",
				IsLastHop:       false,
				IsFault:         true,
				FaultCode:       502,
				FaultPercentage: 50,
			},
			wantErr: false,
		},
		{
			name:    "fault injection - invalid code too low",
			path:    "/fault/399",
			want:    actions{},
			wantErr: true,
		},
		{
			name:    "fault injection - invalid code too high",
			path:    "/fault/600",
			want:    actions{},
			wantErr: true,
		},
		{
			name:    "fault injection - invalid code 200",
			path:    "/fault/200",
			want:    actions{},
			wantErr: true,
		},
		{
			name:    "fault injection - invalid percentage too high",
			path:    "/fault/500/101",
			want:    actions{},
			wantErr: true,
		},
		{
			name:    "fault injection - invalid percentage negative",
			path:    "/fault/500/-1",
			want:    actions{},
			wantErr: true,
		},
		{
			name:    "fault injection - non-numeric code",
			path:    "/fault/abc",
			want:    actions{},
			wantErr: true,
		},
		{
			name:    "fault injection - missing code",
			path:    "/fault/",
			want:    actions{},
			wantErr: true,
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

func TestSendFaultResponse(t *testing.T) {
	logger := createTestLogger()
	handler := NewHandler(30*time.Second, "test-service", logger)

	tests := []struct {
		name           string
		statusCode     int
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "500 Internal Server Error",
			statusCode:     500,
			expectedStatus: 500,
			expectedMsg:    "Fault injected: 500 Internal Server Error",
		},
		{
			name:           "404 Not Found",
			statusCode:     404,
			expectedStatus: 404,
			expectedMsg:    "Fault injected: 404 Not Found",
		},
		{
			name:           "503 Service Unavailable",
			statusCode:     503,
			expectedStatus: 503,
			expectedMsg:    "Fault injected: 503 Service Unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response recorder
			rr := newResponseRecorder()

			// Send fault response
			err := handler.sendFaultResponse(rr, tt.statusCode, logger)
			require.NoError(t, err)

			// Verify status code
			assert.Equal(t, tt.expectedStatus, rr.statusCode)

			// Verify content type
			contentTypes := rr.Header()["Content-Type"]
			require.NotEmpty(t, contentTypes, "Content-Type header should be set")
			assert.Equal(t, "application/json", contentTypes[0])

			// Verify response body contains expected message
			assert.Contains(t, rr.body, tt.expectedMsg)
			assert.Contains(t, rr.body, "test-service")
		})
	}
}

// responseRecorder is a simple HTTP response writer for testing
type responseRecorder struct {
	statusCode int
	header     http.Header
	body       string
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		statusCode: 0,
		header:     make(http.Header),
		body:       "",
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body += string(b)
	return len(b), nil
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
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

func TestHeaderLogging(t *testing.T) {
	logger := createTestLogger()

	tests := []struct {
		name               string
		logHeaders         bool
		inputHeaders       http.Header
		expectedInGroup    bool
		expectedHeaderCount int
	}{
		{
			name:       "headers disabled - empty group",
			logHeaders: false,
			inputHeaders: http.Header{
				"X-Custom-Header": []string{"value1"},
				"Content-Type":    []string{"application/json"},
			},
			expectedInGroup:     false,
			expectedHeaderCount: 0,
		},
		{
			name:       "headers enabled - all headers logged",
			logHeaders: true,
			inputHeaders: http.Header{
				"X-Custom-Header": []string{"value1"},
				"Content-Type":    []string{"application/json"},
			},
			expectedInGroup:     true,
			expectedHeaderCount: 2,
		},
		{
			name:       "headers enabled - empty headers",
			logHeaders: true,
			inputHeaders: http.Header{},
			expectedInGroup:     false,
			expectedHeaderCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(30*time.Second, "test-service", logger, WithHeaderLogging(tt.logHeaders))

			// Test the headersToLogAttrs method
			attr := handler.headersToLogAttrs(tt.inputHeaders, "test_headers")

			// Verify the attribute is a group
			assert.Equal(t, "test_headers", attr.Key)

			// If we expect headers in the group, verify count
			if tt.expectedInGroup {
				group := attr.Value.Group()
				assert.Equal(t, tt.expectedHeaderCount, len(group), "Header count mismatch")
			}
		})
	}
}

func TestHeaderRedaction(t *testing.T) {
	logger := createTestLogger()
	handler := NewHandler(30*time.Second, "test-service", logger, WithHeaderLogging(true))

	tests := []struct {
		name           string
		headerName     string
		headerValue    string
		shouldRedact   bool
	}{
		{
			name:         "Authorization header - should redact",
			headerName:   "Authorization",
			headerValue:  "Bearer secret123",
			shouldRedact: true,
		},
		{
			name:         "Cookie header - should redact",
			headerName:   "Cookie",
			headerValue:  "session=abc123",
			shouldRedact: true,
		},
		{
			name:         "Set-Cookie header - should redact",
			headerName:   "Set-Cookie",
			headerValue:  "session=abc123",
			shouldRedact: true,
		},
		{
			name:         "X-Api-Key header - should redact",
			headerName:   "X-Api-Key",
			headerValue:  "secret-api-key",
			shouldRedact: true,
		},
		{
			name:         "X-Custom-Header - should not redact",
			headerName:   "X-Custom-Header",
			headerValue:  "custom-value",
			shouldRedact: false,
		},
		{
			name:         "Content-Type - should not redact",
			headerName:   "Content-Type",
			headerValue:  "application/json",
			shouldRedact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{
				tt.headerName: []string{tt.headerValue},
			}

			attr := handler.headersToLogAttrs(headers, "test_headers")
			group := attr.Value.Group()

			require.Len(t, group, 1, "Should have exactly one header")

			headerAttr := group[0]
			assert.Equal(t, tt.headerName, headerAttr.Key)

			if tt.shouldRedact {
				assert.Equal(t, "[REDACTED]", headerAttr.Value.String())
			} else {
				assert.Equal(t, tt.headerValue, headerAttr.Value.String())
			}
		})
	}
}

func TestWithHeaderLogging(t *testing.T) {
	logger := createTestLogger()

	tests := []struct {
		name       string
		enabled    bool
		wantEnabled bool
	}{
		{
			name:        "header logging enabled",
			enabled:     true,
			wantEnabled: true,
		},
		{
			name:        "header logging disabled",
			enabled:     false,
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(30*time.Second, "test-service", logger, WithHeaderLogging(tt.enabled))
			assert.Equal(t, tt.wantEnabled, handler.logHeaders)
		})
	}
}

func TestDefaultHeaderLogging(t *testing.T) {
	logger := createTestLogger()

	// Handler created without WithHeaderLogging option should have logHeaders=false by default
	handler := NewHandler(30*time.Second, "test-service", logger)
	assert.False(t, handler.logHeaders, "Default logHeaders should be false")
}
