package main

import (
	"fmt"
	"log"
	"time"

	"github.com/sirfi/termwire/ecr"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("=== Reports Example ===")

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

	// Get available banks
	fmt.Println("=== Getting Available Banks ===")
	banksResp, err := api.GetBanks()
	if err != nil {
		log.Fatalf("Failed to get banks: %v", err)
	}

	fmt.Printf("Code: %s\n", banksResp.Code)
	fmt.Printf("Available banks: %d\n\n", len(banksResp.Banks))
	for i, bank := range banksResp.Banks {
		fmt.Printf("%d. %s (ID: %d)\n", i+1, bank.Name, bank.Id)
	}
	fmt.Println()

	// Get X Report (daily totals without reset)
	fmt.Println("=== X Report (Current Totals) ===")
	xReport, err := api.GetXReport()
	if err != nil {
		log.Fatalf("Failed to get X report: %v", err)
	}

	fmt.Printf("Code: %s\n", xReport.Code)
	fmt.Printf("Report Timestamp: %s\n", xReport.ReportTimestamp)
	fmt.Printf("Total Transactions: %d\n\n", xReport.TransactionCount)

	if len(xReport.SalesTotals) > 0 {
		fmt.Println("Sales Totals:")
		for _, total := range xReport.SalesTotals {
			fmt.Printf("  %s: %.2f (Count: %d)\n",
				total.Currency,
				float64(total.AmountCents)/100.0,
				total.Count)
		}
		fmt.Println()
	}

	if len(xReport.RefundTotals) > 0 {
		fmt.Println("Refund Totals:")
		for _, total := range xReport.RefundTotals {
			fmt.Printf("  %s: %.2f (Count: %d)\n",
				total.Currency,
				float64(total.AmountCents)/100.0,
				total.Count)
		}
		fmt.Println()
	}

	if len(xReport.VoidTotals) > 0 {
		fmt.Println("Void Totals:")
		for _, total := range xReport.VoidTotals {
			fmt.Printf("  %s: %.2f (Count: %d)\n",
				total.Currency,
				float64(total.AmountCents)/100.0,
				total.Count)
		}
		fmt.Println()
	}

	fmt.Printf("Message: %s\n\n", xReport.Message)

	// Get Detailed Report
	fmt.Println("=== Detailed Transaction Report ===")
	
	// Get transactions from last 24 hours
	toTime := time.Now()
	fromTime := toTime.Add(-24 * time.Hour)

	detailedReport, err := api.GetDetailedReport(
		fromTime.Format(time.RFC3339),
		toTime.Format(time.RFC3339),
		100,   // Limit to 100 transactions
		true,  // Include voided transactions
		"",    // All currencies
	)

	if err != nil {
		log.Fatalf("Failed to get detailed report: %v", err)
	}

	fmt.Printf("Code: %s\n", detailedReport.Code)
	fmt.Printf("Total Transactions: %d\n", len(detailedReport.Transactions))
	fmt.Printf("Period: %s to %s\n\n",
		fromTime.Format("2006-01-02 15:04:05"),
		toTime.Format("2006-01-02 15:04:05"))

	if len(detailedReport.Transactions) > 0 {
		fmt.Println("Recent Transactions:")
		fmt.Println(repeatString("-", 100))
		fmt.Printf("%-20s %-10s %-12s %-15s %-20s %-12s\n",
			"Transaction ID", "Type", "Amount", "Card Last 4", "Confirmation", "Timestamp")
		fmt.Println(repeatString("-", 100))

		for _, txn := range detailedReport.Transactions {
			txnType := "SALE"
			switch txn.Type {
			case 1:
				txnType = "REFUND"
			case 2:
				txnType = "VOID"
			}

			timestamp, _ := time.Parse(time.RFC3339, txn.Timestamp)

			fmt.Printf("%-20s %-10s %8.2f %-3s %-15s %-20s %s\n",
				truncateString(txn.TransactionId, 20),
				txnType,
				float64(txn.AmountCents)/100.0,
				txn.Currency,
				truncateString(txn.CardLastFour, 15),
				truncateString(txn.ConfirmationCode, 20),
				timestamp.Format("2006-01-02 15:04"))

			if txn.LoyaltyAmountCents > 0 {
				fmt.Printf("  └─ Loyalty discount: %.2f %s\n",
					float64(txn.LoyaltyAmountCents)/100.0,
					txn.Currency)
			}
		}
		fmt.Println(repeatString("-", 100))
		fmt.Println()
	} else {
		fmt.Println("No transactions found in the specified period.")
	}

	fmt.Printf("Message: %s\n\n", detailedReport.Message)

	// Optionally get Z Report (this will reset the counters on the POS)
	// Uncomment the following to test Z Report
	/*
	fmt.Println("=== Z Report (End of Day - Resets Counters) ===")
	fmt.Println("WARNING: This will reset transaction counters!")
	fmt.Print("Do you want to generate Z Report? (y/N): ")
	
	var response string
	fmt.Scanln(&response)
	
	if response == "y" || response == "Y" {
		zReport, err := api.GetZReport()
		if err != nil {
			log.Fatalf("Failed to get Z report: %v", err)
		}

		fmt.Printf("\nCode: %s\n", zReport.Code)
		fmt.Printf("Report Timestamp: %s\n", zReport.ReportTimestamp)
		fmt.Printf("Z Report Number: %d\n", zReport.ZReportNumber)
		fmt.Printf("Total Transactions: %d\n\n", zReport.TransactionCount)

		if len(zReport.SalesTotals) > 0 {
			fmt.Println("Sales Totals:")
			for _, total := range zReport.SalesTotals {
				fmt.Printf("  %s: %.2f (Count: %d)\n",
					total.Currency,
					float64(total.AmountCents)/100.0,
					total.Count)
			}
			fmt.Println()
		}

		fmt.Printf("Message: %s\n\n", zReport.Message)
		fmt.Println("Note: Transaction counters have been reset.")
	}
	*/

	fmt.Println("Reports example completed successfully!")
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
