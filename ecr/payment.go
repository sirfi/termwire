package ecr

import (
	"fmt"
	"log"
	"strings"
)

// PaymentRequest represents a payment request
type PaymentRequest struct {
	TransactionID    string
	AmountCents      uint32
	Currency         string
	BankID           uint32
	InstallmentCount uint32
	UseLoyalty       bool
	LoyaltyPoints    uint32
}

// PaymentResult represents the result of a payment
type PaymentResult struct {
	Success            bool
	TransactionID      string
	ConfirmationCode   string
	AuthCode           string
	ReceiptNumber      string
	CardAmountCents    uint32
	LoyaltyAmountCents uint32
	RemainingPoints    uint32
	CardNumberMasked   string
	CardHolderName     string
	ErrorMessage       string
	ErrorCode          string
}

// PaymentFlow manages the complete payment flow
type PaymentFlow struct {
	api *API
}

// NewPaymentFlow creates a new payment flow manager
func NewPaymentFlow(api *API) *PaymentFlow {
	return &PaymentFlow{
		api: api,
	}
}

// ExecutePayment executes a complete payment flow
func (pf *PaymentFlow) ExecutePayment(req *PaymentRequest) (*PaymentResult, error) {
	result := &PaymentResult{
		Success:       false,
		TransactionID: req.TransactionID,
	}

	log.Printf("[PAYMENT FLOW] Starting payment - TxnID: %s, Amount: %.2f %s",
		req.TransactionID, float64(req.AmountCents)/100.0, req.Currency)

	// Step 1: Insert card
	log.Println("[PAYMENT FLOW] Step 1: Card insertion")
	cardResp, err := pf.api.InsertCard(req.TransactionID, req.AmountCents, req.Currency)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Card insertion failed: %v", err)
		result.ErrorCode = "CARD_ERROR"
		log.Printf("[PAYMENT FLOW] %s", result.ErrorMessage)
		return result, err
	}

	result.CardNumberMasked = cardResp.CardNumberMasked
	result.CardHolderName = cardResp.CardHolderName

	log.Printf("[PAYMENT FLOW] Card inserted - Number: %s, Holder: %s, Banks: %d",
		cardResp.CardNumberMasked, cardResp.CardHolderName, len(cardResp.AvailableBanks))

	// Step 2: Select bank (use first available if not specified)
	if req.BankID == 0 && len(cardResp.AvailableBanks) > 0 {
		req.BankID = cardResp.AvailableBanks[0].BankId
		log.Printf("[PAYMENT FLOW] Auto-selected bank: %s (ID: %d)",
			cardResp.AvailableBanks[0].BankName, req.BankID)
	}

	// Get AID for selected bank
	var selectedAID string
	for _, bank := range cardResp.AvailableBanks {
		if bank.BankId == req.BankID {
			selectedAID = bank.Aid
			req.UseLoyalty = req.UseLoyalty && bank.SupportsLoyalty
			break
		}
	}

	// Step 3: Process payment
	log.Printf("[PAYMENT FLOW] Step 2: Processing payment - Bank: %d, Loyalty: %v",
		req.BankID, req.UseLoyalty)

	paymentResp, loyaltyResp, err := pf.api.ProcessPayment(
		req.TransactionID,
		req.BankID,
		selectedAID,
		req.InstallmentCount,
		req.UseLoyalty,
	)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Payment processing failed: %v", err)
		result.ErrorCode = "PAYMENT_ERROR"
		log.Printf("[PAYMENT FLOW] %s", result.ErrorMessage)
		return result, err
	}

	// Step 4: Handle loyalty if needed
	if loyaltyResp != nil {
		log.Printf("[PAYMENT FLOW] Step 3: Loyalty inquiry - Points: %d, Value: %.2f %s",
			loyaltyResp.AvailablePoints,
			float64(loyaltyResp.PointsValueCents)/100.0,
			req.Currency)

		// Determine how many points to use
		pointsToUse := req.LoyaltyPoints
		if pointsToUse == 0 {
			// Use all available points if not specified
			pointsToUse = loyaltyResp.AvailablePoints
		}

		// Don't use more points than available
		if pointsToUse > loyaltyResp.AvailablePoints {
			pointsToUse = loyaltyResp.AvailablePoints
		}

		// Calculate points value (assuming 1 point = 1 cent)
		pointsValue := pointsToUse

		// Don't use more value than transaction amount
		if pointsValue > req.AmountCents {
			pointsValue = req.AmountCents
			pointsToUse = pointsValue
		}

		log.Printf("[PAYMENT FLOW] Step 4: Confirming loyalty points - Using: %d (%.2f %s)",
			pointsToUse, float64(pointsValue)/100.0, req.Currency)

		// Confirm loyalty points
		paymentResp, err = pf.api.ConfirmLoyaltyPoints(req.TransactionID, pointsToUse, pointsValue)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("Loyalty confirmation failed: %v", err)
			result.ErrorCode = "LOYALTY_ERROR"
			log.Printf("[PAYMENT FLOW] %s", result.ErrorMessage)
			return result, err
		}
	}

	// Step 5: Payment completed
	if paymentResp != nil {
		result.Success = true
		result.ConfirmationCode = paymentResp.ConfirmationCode
		result.AuthCode = paymentResp.AuthCode
		result.ReceiptNumber = paymentResp.ReceiptNumber
		result.CardAmountCents = paymentResp.CardAmountCents
		result.LoyaltyAmountCents = paymentResp.LoyaltyAmountCents
		result.RemainingPoints = paymentResp.RemainingPoints

		log.Printf("[PAYMENT FLOW] Payment completed successfully")
		log.Printf("[PAYMENT FLOW] Confirmation: %s", result.ConfirmationCode)
		log.Printf("[PAYMENT FLOW] Receipt: %s", result.ReceiptNumber)
		log.Printf("[PAYMENT FLOW] Card Amount: %.2f %s", float64(result.CardAmountCents)/100.0, req.Currency)
		log.Printf("[PAYMENT FLOW] Loyalty Amount: %.2f %s", float64(result.LoyaltyAmountCents)/100.0, req.Currency)
	}

	return result, nil
}

// ExecuteSimplePayment executes a simple payment without loyalty
func (pf *PaymentFlow) ExecuteSimplePayment(transactionID string, amountCents uint32, currency string) (*PaymentResult, error) {
	return pf.ExecutePayment(&PaymentRequest{
		TransactionID: transactionID,
		AmountCents:   amountCents,
		Currency:      currency,
		UseLoyalty:    false,
	})
}

// ExecuteLoyaltyPayment executes a payment with loyalty points
func (pf *PaymentFlow) ExecuteLoyaltyPayment(transactionID string, amountCents uint32, currency string, loyaltyPoints uint32) (*PaymentResult, error) {
	return pf.ExecutePayment(&PaymentRequest{
		TransactionID: transactionID,
		AmountCents:   amountCents,
		Currency:      currency,
		UseLoyalty:    true,
		LoyaltyPoints: loyaltyPoints,
	})
}

// ExecuteInstallmentPayment executes a payment with installments
func (pf *PaymentFlow) ExecuteInstallmentPayment(transactionID string, amountCents uint32, currency string, installments uint32) (*PaymentResult, error) {
	return pf.ExecutePayment(&PaymentRequest{
		TransactionID:    transactionID,
		AmountCents:      amountCents,
		Currency:         currency,
		InstallmentCount: installments,
		UseLoyalty:       false,
	})
}

// PrintReceipt prints a receipt for a payment result
func (pf *PaymentFlow) PrintReceipt(result *PaymentResult, currency string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("                    RECEIPT                    ")
	fmt.Println(strings.Repeat("=", 50))

	if result.Success {
		fmt.Printf("Transaction ID:    %s\n", result.TransactionID)
		fmt.Printf("Receipt Number:    %s\n", result.ReceiptNumber)
		fmt.Printf("Confirmation Code: %s\n", result.ConfirmationCode)
		fmt.Printf("Auth Code:         %s\n", result.AuthCode)
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("Card Number:       %s\n", result.CardNumberMasked)
		fmt.Printf("Card Holder:       %s\n", result.CardHolderName)
		fmt.Println(strings.Repeat("-", 50))

		totalAmount := result.CardAmountCents + result.LoyaltyAmountCents
		fmt.Printf("Total Amount:      %.2f %s\n", float64(totalAmount)/100.0, currency)

		if result.LoyaltyAmountCents > 0 {
			fmt.Printf("Loyalty Discount:  %.2f %s\n", float64(result.LoyaltyAmountCents)/100.0, currency)
			fmt.Printf("Card Amount:       %.2f %s\n", float64(result.CardAmountCents)/100.0, currency)
			fmt.Printf("Remaining Points:  %d\n", result.RemainingPoints)
		}
	} else {
		fmt.Println("           TRANSACTION FAILED           ")
		fmt.Printf("Error Code: %s\n", result.ErrorCode)
		fmt.Printf("Error Message: %s\n", result.ErrorMessage)
	}

	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()
}
