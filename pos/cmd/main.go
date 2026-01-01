package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirfi/termwire/pos"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("=== Termwire POS Terminal ===")

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

	log.Printf("Configuration:")
	log.Printf("  Terminal ID: %s", config.TerminalID)
	log.Printf("  Serial Number: %s", config.SerialNumber)
	log.Printf("  Version: %s", config.Version)
	log.Printf("  Address: %s:%d", config.Host, config.Port)
	log.Printf("  Max Transaction: %.2f", float64(config.MaxTransactionAmount)/100.0)

	// Create and start server
	server := pos.NewServer(config)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Println("POS terminal is ready to accept connections...")
	log.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Print statistics periodically
	go printStatsPeriodically(server)

	<-sigChan
	log.Println("\nReceived shutdown signal")

	// Stop server
	server.Stop()

	log.Println("POS terminal shutdown complete")
}

func printStatsPeriodically(server *pos.Server) {
	// Print stats every 60 seconds
	// In a real implementation, you would use a ticker here
	// For simplicity, we'll skip this for now
}
