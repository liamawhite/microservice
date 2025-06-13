package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/liamawhite/microservice/internal/proxy"
)

func main() {
	port := flag.Int("port", 8080, "Server port")
	timeout := flag.Duration("timeout", 30*time.Second, "Request timeout")
	serviceName := flag.String("service-name", "proxy", "Service name")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	logFormat := flag.String("log-format", "json", "Log format (json, text)")
	flag.Parse()

	// Set up structured logging
	logger := setupLogger(*logLevel, *logFormat, *serviceName)

	logger.Info("Starting microservice", slog.String("service", *serviceName), slog.Int("port", *port), slog.Duration("timeout", *timeout), slog.String("log_level", *logLevel), slog.String("log_format", *logFormat))

	handler := proxy.NewHandler(*timeout, *serviceName, logger)

	mux := http.NewServeMux()
	mux.Handle("/", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Health check request", slog.String("remote_addr", r.RemoteAddr), slog.String("user_agent", r.UserAgent()))
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprint(w, `{"status":"healthy","service":"`+*serviceName+`"}`)
		if err != nil {
			logger.Error("Failed to write health response", slog.String("error", err.Error()))
		}
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	logger.Info("Server listening", slog.String("addr", server.Addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server error", slog.String("error", err.Error()))
		os.Exit(1)
	}
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
		AddSource: true, // Add source file and line number
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
