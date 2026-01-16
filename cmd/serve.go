package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/liamawhite/microservice/internal/proxy"
	"github.com/spf13/cobra"
)

var (
	// Flag variables for serve command
	port        int
	timeout     time.Duration
	serviceName string
	logLevel    string
	logFormat   string
	logHeaders  bool
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP proxy server",
	Long: `Start the HTTP proxy server that creates composable mock microservice topologies.

The server acts as an HTTP proxy that can chain requests through multiple services
to simulate complex microservice architectures. Perfect for testing distributed
system behaviors in development and CI/CD pipelines.

Examples:
  # Start server with default settings
  microservice serve

  # Start on custom port with debug logging
  microservice serve -p 9090 -l debug

  # Configure service name and timeout
  microservice serve -s my-service -t 60s`,
	PreRunE: validateFlags,
	RunE:    runServer,
}

func init() {
	// Define flags with both long and short forms
	serveCmd.Flags().IntVarP(&port, "port", "p", 8080, "HTTP server port")
	serveCmd.Flags().DurationVarP(&timeout, "timeout", "t", 30*time.Second, "Request timeout")
	serveCmd.Flags().StringVarP(&serviceName, "service-name", "s", "proxy", "Service identifier in responses")
	serveCmd.Flags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	serveCmd.Flags().StringVarP(&logFormat, "log-format", "f", "json", "Log output format (json, text)")
	serveCmd.Flags().BoolVar(&logHeaders, "log-headers", false, "Log all request and response headers with sensitive data redaction")
}

// validateFlags validates all flag values before starting the server
func validateFlags(cmd *cobra.Command, args []string) error {
	// Validate port range
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}

	// Validate timeout is positive
	if timeout < 0 {
		return fmt.Errorf("timeout must be positive, got %s", timeout)
	}

	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[logLevel] {
		return fmt.Errorf("log-level must be one of [debug, info, warn, error], got %q", logLevel)
	}

	// Validate log format
	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validFormats[logFormat] {
		return fmt.Errorf("log-format must be one of [json, text], got %q", logFormat)
	}

	return nil
}

// runServer starts the HTTP server with the configured settings
func runServer(cmd *cobra.Command, args []string) error {
	// Set up structured logging
	logger := setupLogger(logLevel, logFormat, serviceName)

	logger.Info("Starting microservice",
		slog.String("service", serviceName),
		slog.Int("port", port),
		slog.Duration("timeout", timeout),
		slog.String("log_level", logLevel),
		slog.String("log_format", logFormat),
		slog.Bool("log_headers", logHeaders),
	)

	handler := proxy.NewHandler(timeout, serviceName, logger, proxy.WithHeaderLogging(logHeaders))

	mux := http.NewServeMux()
	mux.Handle("/", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Health check request",
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
		)
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprint(w, `{"status":"healthy","service":"`+serviceName+`"}`)
		if err != nil {
			logger.Error("Failed to write health response", slog.String("error", err.Error()))
		}
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	logger.Info("Server listening", slog.String("addr", server.Addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server error", slog.String("error", err.Error()))
		return err
	}

	return nil
}

// setupLogger configures and returns a structured logger
func setupLogger(level, format, serviceName string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true,
	}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)

	// Add service name to all log entries
	return logger.With(slog.String("service", serviceName))
}
