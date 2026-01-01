package main

import (
	"fmt"
	"log"

	"github.com/sirfi/termwire/ecr"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("=== Refund and Void Example ===")

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

	// Create payment flow
	paymentFlow := ecr.NewPaymentFlow(api)

	// First, make a payment that we'll refund/void
	fmt.Println("=== Step 1: Create Original Transaction ===")
	fmt.Println("Amount: 250.00 TRY")
	fmt.Println("Transaction ID: TXN-ORIGINAL-001")

	originalResult, err := paymentFlow.ExecuteSimplePayment(
		"TXN-ORIGINAL-001",
		25000,
		"TRY",
	)

	if err != nil {
		log.Fatalf("Original payment failed: %v", err)
	}

	fmt.Println("\n=== Original Transaction Successful ===")
	paymentFlow.UpdatedPrintReceipt(originalResult, "TRY")

	originalTxnID := originalResult.TransactionID
	originalConfCode := originalResult.ConfirmationCode

	// Example 1: Partial Refund
	fmt.Println("\n=== Step 2: Partial Refund ===")
	fmt.Println("Refunding 100.00 TRY from original transaction")
	fmt.Printf("Original Transaction: %s\n", originalTxnID)
	fmt.Printf("Original Confirmation: %s\n\n", originalConfCode)

	refundResp, err := api.RefundTransaction(
		originalTxnID,
		originalConfCode,
		10000, // Refund 100.00 TRY
		"TRY",
		"Customer requested partial refund",
	)

	if err != nil {
		log.Printf("Refund failed: %v\n", err)
	} else {
		fmt.Println("=== Refund Successful ===")
		fmt.Println(repeatString("=", 50))
		fmt.Printf("Refund Transaction ID:  %s\n", refundResp.TransactionId)
		fmt.Printf("Refund Amount:          %.2f %s\n",
			float64(refundResp.RefundAmountCents)/100.0,
			refundResp.Currency)
		fmt.Printf("Confirmation Code:      %s\n", refundResp.ConfirmationCode)
		fmt.Printf("Message:                %s\n", refundResp.Message)
		fmt.Println(repeatString("=", 50))
		fmt.Println()
	}

	// Create another transaction for void example
	fmt.Println("\n=== Step 3: Create Another Transaction for Void ===")
	fmt.Println("Amount: 175.00 TRY")
	fmt.Println("Transaction ID: TXN-VOID-001")

	voidOriginalResult, err := paymentFlow.ExecuteSimplePayment(
		"TXN-VOID-001",
		17500,
		"TRY",
	)

	if err != nil {
		log.Fatalf("Second payment failed: %v", err)
	}

	fmt.Println("\n=== Second Transaction Successful ===")
	paymentFlow.UpdatedPrintReceipt(voidOriginalResult, "TRY")

	voidTxnID := voidOriginalResult.TransactionID
	voidConfCode := voidOriginalResult.ConfirmationCode

	// Example 2: Void Transaction
	fmt.Println("\n=== Step 4: Void Transaction ===")
	fmt.Println("Voiding entire transaction")
	fmt.Printf("Transaction to Void: %s\n", voidTxnID)
	fmt.Printf("Original Confirmation: %s\n\n", voidConfCode)

	voidResp, err := api.VoidTransaction(
		voidTxnID,
		voidConfCode,
		"TRY",
		"Transaction cancelled by customer",
	)

	if err != nil {
		log.Printf("Void failed: %v\n", err)
	} else {
		fmt.Println("=== Void Successful ===")
		fmt.Println(repeatString("=", 50))
		fmt.Printf("Void Transaction ID:    %s\n", voidResp.TransactionId)
		fmt.Printf("Original Transaction:   %s\n", voidTxnID)
		fmt.Printf("Confirmation Code:      %s\n", voidResp.ConfirmationCode)
		fmt.Printf("Message:                %s\n", voidResp.Message)
		fmt.Println(repeatString("=", 50))
		fmt.Println()
	}

	// Example 3: Full Refund
	fmt.Println("\n=== Step 5: Create Transaction for Full Refund ===")
	fmt.Println("Amount: 300.00 TRY")
	fmt.Println("Transaction ID: TXN-FULLREFUND-001")

	fullRefundOriginal, err := paymentFlow.ExecuteSimplePayment(
		"TXN-FULLREFUND-001",
		30000,
		"TRY",
	)

	if err != nil {
		log.Fatalf("Third payment failed: %v", err)
	}

	fmt.Println("\n=== Third Transaction Successful ===")
	paymentFlow.UpdatedPrintReceipt(fullRefundOriginal, "TRY")

	// Full refund
	fmt.Println("\n=== Step 6: Full Refund ===")
	fmt.Println("Refunding full amount")
	fmt.Printf("Original Transaction: %s\n", fullRefundOriginal.TransactionID)
	fmt.Printf("Amount to Refund: %.2f TRY\n\n",
		float64(fullRefundOriginal.CardAmountCents+fullRefundOriginal.LoyaltyAmountCents)/100.0)

	fullRefundResp, err := api.RefundTransaction(
		fullRefundOriginal.TransactionID,
		fullRefundOriginal.ConfirmationCode,
		fullRefundOriginal.CardAmountCents+fullRefundOriginal.LoyaltyAmountCents,
		"TRY",
		"Customer returned all items",
	)

	if err != nil {
		log.Printf("Full refund failed: %v\n", err)
	} else {
		fmt.Println("=== Full Refund Successful ===")
		fmt.Println(repeatString("=", 50))
		fmt.Printf("Refund Transaction ID:  %s\n", fullRefundResp.TransactionId)
		fmt.Printf("Refund Amount:          %.2f %s\n",
			float64(fullRefundResp.RefundAmountCents)/100.0,
			fullRefundResp.Currency)
		fmt.Printf("Confirmation Code:      %s\n", fullRefundResp.ConfirmationCode)
		fmt.Printf("Message:                %s\n", fullRefundResp.Message)
		fmt.Println(repeatString("=", 50))
		fmt.Println()
	}

	// Summary
	fmt.Println("\n=== Summary ===")
	fmt.Println("Completed operations:")
	fmt.Println("1. Original payment: 250.00 TRY")
	fmt.Println("   - Partial refund: 100.00 TRY")
	fmt.Println("2. Second payment: 175.00 TRY")
	fmt.Println("   - Voided: 175.00 TRY")
	fmt.Println("3. Third payment: 300.00 TRY")
	fmt.Println("   - Full refund: 300.00 TRY")
	fmt.Println()

	fmt.Println("Refund and void examples completed successfully!")
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
