package main

import (
	"fmt"
	"log"
	"time"

	"github.com/sirfi/termwire/ecr"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("=== Loyalty Payment Example ===")

	// Create ECR configuration with loyalty enabled
	config := ecr.DefaultConfig().
		WithPOSAddress("localhost", 8080).
		WithLoyalty(true).
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

	ts := time.Now().UnixMilli()
	txn1 := fmt.Sprintf("TXN-LOYALTY-1-%d", ts)
	txn2 := fmt.Sprintf("TXN-LOYALTY-2-%d", ts)
	txn3 := fmt.Sprintf("TXN-LOYALTY-3-%d", ts)

	// Example 1: Payment with automatic loyalty point usage
	fmt.Println("=== Payment 1: Automatic Loyalty Points ===")
	fmt.Println("Amount: 200.00 TRY")
	fmt.Printf("Transaction ID: %s\n", txn1)
	fmt.Println("Loyalty: Use all available points")

	result1, err := paymentFlow.ExecuteLoyaltyPayment(
		txn1,  // Transaction ID
		20000, // Amount in cents (200.00 TRY)
		"TRY", // Currency
		0,     // Use all available points (0 = auto)
	)

	if err != nil {
		log.Printf("Payment 1 failed: %v", err)
		if result1 != nil {
			paymentFlow.PrintReceipt(result1, "TRY")
		}
	} else {
		fmt.Println("\n=== Payment 1 Successful ===")
		paymentFlow.PrintReceipt(result1, "TRY")
	}

	// Wait a bit before next transaction
	fmt.Println("\nWaiting before next transaction...")
	// time.Sleep(2 * time.Second)

	// Example 2: Payment with specific loyalty points
	fmt.Println("=== Payment 2: Specific Loyalty Points ===")
	fmt.Println("Amount: 350.00 TRY")
	fmt.Printf("Transaction ID: %s\n", txn2)
	fmt.Println("Loyalty: Use 5000 points (50.00 TRY)")

	result2, err := paymentFlow.ExecuteLoyaltyPayment(
		txn2,  // Transaction ID
		35000, // Amount in cents (350.00 TRY)
		"TRY", // Currency
		5000,  // Use 5000 points
	)

	if err != nil {
		log.Printf("Payment 2 failed: %v", err)
		if result2 != nil {
			paymentFlow.PrintReceipt(result2, "TRY")
		}
	} else {
		fmt.Println("\n=== Payment 2 Successful ===")
		paymentFlow.PrintReceipt(result2, "TRY")
	}

	// Example 3: Payment using manual flow for more control
	fmt.Println("\n=== Payment 3: Manual Loyalty Flow ===")
	fmt.Println("Amount: 500.00 TRY")
	fmt.Printf("Transaction ID: %s\n", txn3)

	transactionID := txn3
	amountCents := uint32(50000) // 500.00 TRY

	// Step 1: Insert card
	fmt.Println("Step 1: Inserting card...")
	cardResp, err := api.InsertCard(transactionID, amountCents, "TRY")
	if err != nil {
		log.Fatalf("Card insertion failed: %v", err)
	}
	fmt.Printf("Card: %s, Holder: %s\n", cardResp.CardNumberMasked, cardResp.CardHolderName)

	// Step 2: Select bank and check loyalty
	if len(cardResp.AvailableBanks) == 0 {
		log.Fatal("No banks available on card")
	}

	selectedBank := cardResp.AvailableBanks[0]
	fmt.Printf("\nStep 2: Selected bank: %s\n", selectedBank.BankName)
	fmt.Printf("Supports loyalty: %v\n", selectedBank.SupportsLoyalty)

	// Step 3: Process payment with loyalty
	fmt.Println("\nStep 3: Processing payment with loyalty...")
	paymentResp, loyaltyResp, err := api.ProcessPayment(
		transactionID,
		selectedBank.BankId,
		selectedBank.Aid,
		0, // No installments
		selectedBank.SupportsLoyalty,
	)

	if err != nil {
		log.Fatalf("Payment processing failed: %v", err)
	}

	// Step 4: Handle loyalty inquiry if received
	if loyaltyResp != nil {
		fmt.Printf("\nStep 4: Loyalty inquiry response\n")
		fmt.Printf("Available points: %d\n", loyaltyResp.AvailablePoints)
		fmt.Printf("Points value: %.2f TRY\n", float64(loyaltyResp.PointsValueCents)/100.0)

		// Use 75% of available points
		pointsToUse := loyaltyResp.AvailablePoints * 75 / 100
		pointsValue := pointsToUse // Assuming 1 point = 1 cent

		// Don't use more value than transaction amount
		if pointsValue > amountCents {
			pointsValue = amountCents
			pointsToUse = pointsValue
		}

		fmt.Printf("Using %d points (%.2f TRY)\n", pointsToUse, float64(pointsValue)/100.0)

		// Step 5: Confirm loyalty points
		fmt.Println("\nStep 5: Confirming loyalty points...")
		paymentResp, err = api.ConfirmLoyaltyPoints(transactionID, pointsToUse, pointsValue)
		if err != nil {
			log.Fatalf("Loyalty confirmation failed: %v", err)
		}
	}

	// Display result
	if paymentResp != nil {
		result3 := &ecr.PaymentResult{
			Success:            true,
			TransactionID:      transactionID,
			ConfirmationCode:   paymentResp.ConfirmationCode,
			AuthCode:           paymentResp.AuthCode,
			ReceiptNumber:      paymentResp.ReceiptNumber,
			CardAmountCents:    paymentResp.CardAmountCents,
			LoyaltyAmountCents: paymentResp.LoyaltyAmountCents,
			RemainingPoints:    paymentResp.RemainingPoints,
			CardNumberMasked:   cardResp.CardNumberMasked,
			CardHolderName:     cardResp.CardHolderName,
		}

		fmt.Println("\n=== Payment 3 Successful ===")
		paymentFlow.PrintReceipt(result3, "TRY")
	}

	fmt.Println("\nLoyalty payment examples completed successfully!")
}
