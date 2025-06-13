package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Handler handles HTTP proxy requests
type Handler struct {
	client      *http.Client
	timeout     time.Duration
	serviceName string
	logger      *slog.Logger
}

// Response represents the standard response format
type Response struct {
	Status  int    `json:"status"`
	Service string `json:"service"`
	Message string `json:"message,omitempty"`
}

// NewHandler creates a new proxy handler with structured logging
func NewHandler(timeout time.Duration, serviceName string, logger *slog.Logger) *Handler {
	return &Handler{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout:     timeout,
		serviceName: serviceName,
		logger:      logger,
	}
}

// actions represents the parsed proxy path actions
type actions struct {
	NextHop   string // The next hop service and port to forward to
	Remaining string // The remaining path after next hop
	IsLastHop bool   // Whether this is the last hop in the chain
}

// parsePath validates and parses the proxy path into actions
// Returns the actions to take and any error
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

	// Path must start with /proxy/
	if !strings.HasPrefix(path, "/proxy/") {
		return actions{}, fmt.Errorf("invalid path: must start with /proxy/")
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
	logger.Info("Incoming request", slog.String("user_agent", r.UserAgent()), slog.String("query", r.URL.RawQuery))

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
		logger.Info("Request completed", slog.Duration("duration", duration), slog.Int("status_code", http.StatusOK))
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
	logger.Info("Request completed", slog.Duration("total_duration", totalDuration), slog.Duration("forward_duration", forwardDuration), slog.Int("status_code", nextResp.StatusCode))
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
