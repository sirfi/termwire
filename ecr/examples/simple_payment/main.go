package main

import (
	"fmt"
	"log"

	"github.com/sirfi/termwire/ecr"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("=== Simple Payment Example ===")

	// Create ECR configuration
	config := ecr.DefaultConfig().
		WithPOSAddress("localhost", 8080).
		WithDebug(true)

	// Create API instance
	api := ecr.NewAPI(config)

	// Connect to POS terminal
	fmt.Println("Connecting to POS terminal...")
	if err := api.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer api.Disconnect()

	fmt.Println("Connected successfully!")

	// Test connection with ping
	fmt.Println("Testing connection...")
	if err := api.Ping(); err != nil {
		log.Fatalf("Ping failed: %v", err)
	}
	fmt.Println("Ping successful!")

	// Get terminal info
	fmt.Println("Getting terminal information...")
	termInfo, err := api.GetTerminalInfo()
	if err != nil {
		log.Fatalf("Failed to get terminal info: %v", err)
	}
	fmt.Printf("Terminal Version: %s\n", termInfo.Version)
	fmt.Printf("Serial Number: %s\n\n", termInfo.SerialNumber)

	// Create payment flow
	paymentFlow := ecr.NewPaymentFlow(api)

	// Execute a simple payment
	fmt.Println("=== Processing Payment ===")
	fmt.Println("Amount: 150.00 TRY")
	fmt.Println("Transaction ID: TXN-SIMPLE-001")

	result, err := paymentFlow.ExecuteSimplePayment(
		"TXN-SIMPLE-001",  // Transaction ID
		15000,             // Amount in cents (150.00 TRY)
		"TRY",             // Currency
	)

	if err != nil {
		log.Printf("Payment failed: %v", err)
		if result != nil {
			paymentFlow.UpdatedPrintReceipt(result, "TRY")
		}
		return
	}

	// Print receipt
	fmt.Println("\n=== Payment Successful ===")
	paymentFlow.UpdatedPrintReceipt(result, "TRY")

	fmt.Println("Simple payment example completed successfully!")
}
