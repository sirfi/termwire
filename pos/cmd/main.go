package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirfi/termwire/pos"
)

func main() {
	// Load configuration
	config := pos.DefaultConfig()

	// Override with environment variables if set
	if host := os.Getenv("POS_HOST"); host != "" {
		config.Host = host
	}
	if port := os.Getenv("POS_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &config.Port)
	}
	if terminalID := os.Getenv("POS_TERMINAL_ID"); terminalID != "" {
		config.TerminalID = terminalID
	}
	if os.Getenv("POS_TLS_ENABLED") == "true" {
		config.TLSEnabled = true
		config.TLSCertFile = getEnv("POS_TLS_CERT", "certs/server.crt")
		config.TLSKeyFile = getEnv("POS_TLS_KEY", "certs/server.key")
		config.TLSCAFile = getEnv("POS_TLS_CA", "certs/ca.crt")
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: config.LogLevel}))
	logger.Info("starting Termwire POS terminal",
		slog.String("terminal_id", config.TerminalID),
		slog.String("serial", config.SerialNumber),
		slog.String("version", config.Version),
		slog.String("addr", fmt.Sprintf("%s:%d", config.Host, config.Port)),
		slog.Bool("tls", config.TLSEnabled),
	)

	// Create and start server
	server := pos.NewServer(config)
	if err := server.Start(); err != nil {
		logger.Error("failed to start server", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("POS terminal ready — press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go printStatsPeriodically(server, logger)

	<-sigChan
	logger.Info("received shutdown signal")
	server.Stop()
	logger.Info("POS terminal shutdown complete")
}

func printStatsPeriodically(server *pos.Server, logger *slog.Logger) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := server.GetStats()
		logger.Info("server statistics", slog.Any("stats", stats))
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
