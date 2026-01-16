package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Handler handles HTTP proxy requests
type Handler struct {
	client      *http.Client
	timeout     time.Duration
	serviceName string
	logger      *slog.Logger
	logHeaders  bool
}

// Response represents the standard response format
type Response struct {
	Status  int    `json:"status"`
	Service string `json:"service"`
	Message string `json:"message,omitempty"`
}

// HandlerOption configures a Handler
type HandlerOption func(*Handler)

// WithHeaderLogging enables or disables request/response header logging
func WithHeaderLogging(enabled bool) HandlerOption {
	return func(h *Handler) {
		h.logHeaders = enabled
	}
}

// NewHandler creates a new proxy handler with structured logging
func NewHandler(timeout time.Duration, serviceName string, logger *slog.Logger, opts ...HandlerOption) *Handler {
	h := &Handler{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout:     timeout,
		serviceName: serviceName,
		logger:      logger,
		logHeaders:  false, // default to false
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	return h
}

// actions represents the parsed proxy path actions
type actions struct {
	NextHop         string // The next hop service and port to forward to
	Remaining       string // The remaining path after next hop
	IsLastHop       bool   // Whether this is the last hop in the chain
	IsFault         bool   // Whether this is a fault injection
	FaultCode       int    // HTTP status code to inject (400-599)
	FaultPercentage int    // Percentage chance of fault triggering (0-100)
}

// sensitiveHeaders lists headers that should be redacted in logs for security
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"cookie":              true,
	"set-cookie":          true,
	"proxy-authorization": true,
	"x-api-key":           true,
	"x-auth-token":        true,
}

// headersToLogAttrs converts HTTP headers to slog.Attr with sensitive header redaction
func (h *Handler) headersToLogAttrs(headers http.Header, prefix string) slog.Attr {
	if !h.logHeaders || len(headers) == 0 {
		return slog.Group(prefix) // Empty group if logging disabled
	}

	attrs := make([]any, 0, len(headers))
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		value := strings.Join(values, ", ")

		if sensitiveHeaders[lowerKey] {
			value = "[REDACTED]"
		}

		attrs = append(attrs, slog.String(key, value))
	}

	return slog.Group(prefix, attrs...)
}

// parsePath validates and parses the proxy path into actions
// Returns the actions to take and any error
// Supports both /proxy/ and /fault/ segments:
// - /proxy/service:port - forward to next service
// - /fault/500 - always inject 500 error
// - /fault/500/30 - inject 500 error 30% of the time
func parsePath(path string) (actions, error) {
	if path == "" || path == "/" {
		return actions{
			NextHop:   "",
			Remaining: "/",
			IsLastHop: true,
		}, nil
	}

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return actions{}, fmt.Errorf("invalid path: missing service")
	}

	// Check if this is a fault injection path
	if strings.HasPrefix(path, "/fault/") {
		if len(parts) < 3 {
			return actions{}, fmt.Errorf("invalid fault path: must be /fault/<code> or /fault/<code>/<percentage>")
		}

		// Parse status code
		statusCode, err := strconv.Atoi(parts[2])
		if err != nil {
			return actions{}, fmt.Errorf("invalid fault code: must be a number")
		}

		// Validate status code is 400-599
		if statusCode < 400 || statusCode > 599 {
			return actions{}, fmt.Errorf("invalid fault code: must be 400-599")
		}

		// Default percentage to 100
		percentage := 100

		// Check if percentage is provided
		startIdx := 3
		if len(parts) > 3 && parts[3] != "" {
			// Try to parse as percentage
			if p, err := strconv.Atoi(parts[3]); err == nil {
				percentage = p
				startIdx = 4
			}
		}

		// Validate percentage is 0-100
		if percentage < 0 || percentage > 100 {
			return actions{}, fmt.Errorf("invalid fault percentage: must be 0-100")
		}

		// Get remaining path
		var remaining string
		if len(parts) > startIdx {
			remaining = "/" + strings.Join(parts[startIdx:], "/")
		} else {
			remaining = "/"
		}

		return actions{
			NextHop:         "",
			Remaining:       remaining,
			IsLastHop:       false,
			IsFault:         true,
			FaultCode:       statusCode,
			FaultPercentage: percentage,
		}, nil
	}

	// Path must start with /proxy/
	if !strings.HasPrefix(path, "/proxy/") {
		return actions{}, fmt.Errorf("invalid path: must start with /proxy/ or /fault/")
	}

	// Get the first service
	nextHop := parts[2]
	if nextHop == "" {
		return actions{}, fmt.Errorf("invalid path: empty service name")
	}

	// If there's more path, it's the remaining path
	var remaining string
	if len(parts) > 3 {
		remaining = "/" + strings.Join(parts[3:], "/")
	} else {
		remaining = "/"
	}

	return actions{
		NextHop:   nextHop,
		Remaining: remaining,
		IsLastHop: false,
	}, nil
}

// ServeHTTP handles incoming HTTP requests with comprehensive logging
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := fmt.Sprintf("%d", startTime.UnixNano())

	// Create logger with request context
	logger := h.logger.With(slog.String("request_id", requestID), slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.String("service", h.serviceName), slog.String("remote_addr", r.RemoteAddr))
	logger.Info("Incoming request",
		slog.String("user_agent", r.UserAgent()),
		slog.String("query", r.URL.RawQuery),
		h.headersToLogAttrs(r.Header, "request_headers"))

	// Parse the current hop from the path
	actions, err := parsePath(r.URL.Path)
	if err != nil {
		logger.Error("Path parsing failed", slog.String("error", err.Error()), slog.String("path", r.URL.Path))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logger.Debug("Path parsed successfully", slog.String("next_hop", actions.NextHop), slog.String("remaining", actions.Remaining), slog.Bool("is_last_hop", actions.IsLastHop))

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	// Handle fault injection
	if actions.IsFault {
		logger.Info("Fault injection detected", slog.Int("fault_code", actions.FaultCode), slog.Int("percentage", actions.FaultPercentage))

		// Determine if fault should trigger based on percentage
		shouldTrigger := rand.Intn(100) < actions.FaultPercentage

		if shouldTrigger {
			logger.Info("Fault triggered", slog.Int("fault_code", actions.FaultCode))

			if err := h.sendFaultResponse(w, actions.FaultCode, logger); err != nil {
				logger.Error("Failed to send fault response", slog.String("error", err.Error()))
				http.Error(w, fmt.Sprintf("Response error: %v", err), http.StatusInternalServerError)
				return
			}

			duration := time.Since(startTime)
			logger.Info("Fault injection completed",
				slog.Duration("duration", duration),
				slog.Int("status_code", actions.FaultCode),
				h.headersToLogAttrs(w.Header(), "response_headers"))
			return
		}

		logger.Info("Fault not triggered, continuing to next segment", slog.String("remaining", actions.Remaining))

		// Fault didn't trigger, continue processing remaining path
		// If there's a remaining path, process it recursively
		if actions.Remaining != "/" {
			// Parse and process the remaining path
			nextActions, err := parsePath(actions.Remaining)
			if err != nil {
				logger.Error("Failed to parse remaining path", slog.String("error", err.Error()))
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			actions = nextActions
			logger.Debug("Continuing with remaining path", slog.String("next_hop", actions.NextHop), slog.String("remaining", actions.Remaining))
		} else {
			// No remaining path, return success
			logger.Info("No remaining path, returning success")
			if err := h.sendFinalResponse(w, http.StatusOK, logger); err != nil {
				logger.Error("Failed to send final response", slog.String("error", err.Error()))
				http.Error(w, fmt.Sprintf("Response error: %v", err), http.StatusInternalServerError)
				return
			}
			duration := time.Since(startTime)
			logger.Info("Request completed", slog.Duration("duration", duration), slog.Int("status_code", http.StatusOK))
			return
		}
	}

	// If this is the last hop, we're done
	if actions.IsLastHop {
		logger.Info("Processing as final hop")

		// Create our own response since we're the final destination
		if err := h.sendFinalResponse(w, http.StatusOK, logger); err != nil {
			logger.Error("Failed to send final response", slog.String("error", err.Error()))
			http.Error(w, fmt.Sprintf("Response error: %v", err), http.StatusInternalServerError)
			return
		}

		duration := time.Since(startTime)
		logger.Info("Request completed",
			slog.Duration("duration", duration),
			slog.Int("status_code", http.StatusOK),
			h.headersToLogAttrs(w.Header(), "response_headers"))
		return
	}

	// Construct the next hop URL with port, using only the remaining path
	nextHopURL := fmt.Sprintf("http://%s%s", actions.NextHop, actions.Remaining)

	logger.Info("Forwarding to next hop", slog.String("next_hop_url", nextHopURL), slog.String("next_service", actions.NextHop))

	// Forward to next hop
	nextReq, err := http.NewRequestWithContext(ctx, r.Method, nextHopURL, r.Body)
	if err != nil {
		logger.Error("Failed to create next hop request", slog.String("error", err.Error()), slog.String("next_hop_url", nextHopURL))
		http.Error(w, fmt.Sprintf("Failed to create next hop request: %v", err), http.StatusInternalServerError)
		return
	}

	forwardStartTime := time.Now()

	// Forward to the next hop
	nextResp, err := h.client.Do(nextReq)
	if err != nil {
		forwardDuration := time.Since(forwardStartTime)
		logger.Error("Next hop request failed", slog.String("error", err.Error()), slog.String("next_hop_url", nextHopURL), slog.Duration("forward_duration", forwardDuration))
		http.Error(w, fmt.Sprintf("Next hop error: %v", err), http.StatusBadGateway)
		return
	}
	defer func() { _ = nextResp.Body.Close() }()

	forwardDuration := time.Since(forwardStartTime)
	logger.Info("Next hop response received", slog.Int("status_code", nextResp.StatusCode), slog.Duration("forward_duration", forwardDuration), slog.String("next_hop_url", nextHopURL))

	// Forward the downstream response as-is (don't modify the service field)
	if err := h.forwardResponse(w, nextResp, logger); err != nil {
		logger.Error("Failed to forward response", slog.String("error", err.Error()), slog.Int("upstream_status", nextResp.StatusCode))
		http.Error(w, fmt.Sprintf("Response error: %v", err), http.StatusInternalServerError)
		return
	}

	totalDuration := time.Since(startTime)
	logger.Info("Request completed",
		slog.Duration("total_duration", totalDuration),
		slog.Duration("forward_duration", forwardDuration),
		slog.Int("status_code", nextResp.StatusCode),
		h.headersToLogAttrs(nextResp.Header, "upstream_headers"),
		h.headersToLogAttrs(w.Header(), "response_headers"))
}

// sendFinalResponse creates and sends our own response when we're the final destination
func (h *Handler) sendFinalResponse(w http.ResponseWriter, statusCode int, logger *slog.Logger) error {
	logger.Debug("Sending final response", slog.Int("status_code", statusCode), slog.String("service", h.serviceName))

	response := Response{
		Status:  statusCode,
		Service: h.serviceName,
		Message: "Request processed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode JSON response", slog.String("error", err.Error()))
		return err
	}

	logger.Debug("Final response sent successfully")
	return nil
}

// sendFaultResponse creates and sends a fault injection response
func (h *Handler) sendFaultResponse(w http.ResponseWriter, statusCode int, logger *slog.Logger) error {
	logger.Debug("Sending fault response", slog.Int("status_code", statusCode), slog.String("service", h.serviceName))

	// Get standard HTTP status text
	statusText := http.StatusText(statusCode)
	if statusText == "" {
		statusText = "Unknown Error"
	}

	response := Response{
		Status:  statusCode,
		Service: h.serviceName,
		Message: fmt.Sprintf("Fault injected: %d %s", statusCode, statusText),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode JSON fault response", slog.String("error", err.Error()))
		return err
	}

	logger.Debug("Fault response sent successfully")
	return nil
}

// forwardResponse forwards the downstream response as-is without modification
func (h *Handler) forwardResponse(w http.ResponseWriter, resp *http.Response, logger *slog.Logger) error {
	logger.Debug("Forwarding response", slog.Int("status_code", resp.StatusCode), slog.Int("header_count", len(resp.Header)))

	// Copy headers from downstream response
	headerCount := 0
	for k, v := range resp.Header {
		for _, val := range v {
			w.Header().Add(k, val)
			headerCount++
		}
	}

	w.WriteHeader(resp.StatusCode)

	// Copy the response body as-is
	_, err := io.Copy(w, resp.Body)
	if err != nil {
		logger.Error("Failed to copy response body", slog.String("error", err.Error()))
		return err
	}

	logger.Debug("Response forwarded successfully", slog.Int("headers_copied", headerCount))

	return nil
}
