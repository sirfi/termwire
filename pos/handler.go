package pos

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	pb "github.com/sirfi/termwire/protocol"
)

// MessageHandler handles incoming protocol messages
type MessageHandler struct {
	config     *Config
	txnManager *TransactionManager
	cardReader *CardReader
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(config *Config) *MessageHandler {
	return &MessageHandler{
		config:     config,
		txnManager: NewTransactionManager(config),
		cardReader: NewCardReader(),
	}
}

// HandleMessage processes an incoming message and returns a response
func (h *MessageHandler) HandleMessage(msg *pb.Message) (*pb.Message, error) {
	log.Printf("[HANDLER] Processing message ID: %s, Type: %T", msg.MessageId, msg.Body)

	switch body := msg.Body.(type) {
	case *pb.Message_GetVersionRequest:
		return h.handleGetVersion(msg)
	case *pb.Message_GetTerminalInfoRequest:
		return h.handleGetTerminalInfo(msg)
	case *pb.Message_GetBanksRequest:
		return h.handleGetBanks(msg)
	case *pb.Message_CardInsertionRequest:
		return h.handleCardInsertion(msg, body.CardInsertionRequest)
	case *pb.Message_PaymentProcessingRequest:
		return h.handlePaymentProcessing(msg, body.PaymentProcessingRequest)
	case *pb.Message_LoyaltyPointsConfirmation:
		return h.handleLoyaltyConfirmation(msg, body.LoyaltyPointsConfirmation)
	case *pb.Message_PaymentCompletionRequest:
		return h.handlePaymentCompletion(msg, body.PaymentCompletionRequest)
	case *pb.Message_RefundTransactionRequest:
		return h.handleRefund(msg, body.RefundTransactionRequest)
	case *pb.Message_VoidTransactionRequest:
		return h.handleVoid(msg, body.VoidTransactionRequest)
	case *pb.Message_XReportRequest:
		return h.handleXReport(msg)
	case *pb.Message_ZReportRequest:
		return h.handleZReport(msg)
	case *pb.Message_DetailedReportRequest:
		return h.handleDetailedReport(msg, body.DetailedReportRequest)
	case *pb.Message_GiftCardInquiryRequest:
		return h.handleGiftCardInquiry(msg, body.GiftCardInquiryRequest)
	case *pb.Message_GiftCardChargeRequest:
		return h.handleGiftCardCharge(msg, body.GiftCardChargeRequest)
	default:
		return nil, fmt.Errorf("unsupported message type: %T", msg.Body)
	}
}

// handleGetVersion handles version request
func (h *MessageHandler) handleGetVersion(msg *pb.Message) (*pb.Message, error) {
	response := &pb.GetVersionResponse{
		Code:    "00",
		Version: h.config.Version,
	}

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GetVersionResponse{
			GetVersionResponse: response,
		},
	}, nil
}

// handleGetTerminalInfo handles terminal info request
func (h *MessageHandler) handleGetTerminalInfo(msg *pb.Message) (*pb.Message, error) {
	response := &pb.GetTerminalInfoResponse{
		Code:         "00",
		Version:      h.config.Version,
		SerialNumber: h.config.SerialNumber,
	}

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GetTerminalInfoResponse{
			GetTerminalInfoResponse: response,
		},
	}, nil
}

// handleGetBanks handles banks request
func (h *MessageHandler) handleGetBanks(msg *pb.Message) (*pb.Message, error) {
	response := &pb.GetBanksResponse{
		Code:  "00",
		Banks: GetBanks(),
	}

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GetBanksResponse{
			GetBanksResponse: response,
		},
	}, nil
}

// handleCardInsertion handles card insertion request
func (h *MessageHandler) handleCardInsertion(msg *pb.Message, req *pb.CardInsertionRequest) (*pb.Message, error) {
	log.Printf("[HANDLER] Card insertion request - Amount: %d %s, TxnID: %s",
		req.TransactionAmountCents, req.Currency, req.TransactionId)

	// Create transaction
	_, err := h.txnManager.CreateTransaction(req.TransactionId, req.TransactionAmountCents, req.Currency)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "01", err.Error()), nil
	}

	// Simulate card insertion
	card := h.cardReader.SimulateCardInsertion()
	h.txnManager.SetCardData(req.TransactionId, card)
	h.txnManager.UpdateTransactionState(req.TransactionId, StateBankSelection)

	log.Printf("[HANDLER] Card inserted - Number: %s, Holder: %s, Banks: %d",
		card.MaskedNumber, card.HolderName, len(card.AvailableBanks))

	response := &pb.CardInsertionResponse{
		Code:             "00",
		CardNumberMasked: card.MaskedNumber,
		CardHolderName:   card.HolderName,
		AvailableBanks:   card.AvailableBanks,
		Message:          "Card read successfully. Please select bank.",
	}

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_CardInsertionResponse{
			CardInsertionResponse: response,
		},
	}, nil
}

// handlePaymentProcessing handles payment processing request
func (h *MessageHandler) handlePaymentProcessing(msg *pb.Message, req *pb.PaymentProcessingRequest) (*pb.Message, error) {
	log.Printf("[HANDLER] Payment processing - TxnID: %s, BankID: %d, UseLoyalty: %v",
		req.TransactionId, req.SelectedBankId, req.UseLoyaltyPoints)

	txn, err := h.txnManager.GetTransaction(req.TransactionId)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "02", err.Error()), nil
	}

	// Set bank selection
	h.txnManager.SetBankSelection(req.TransactionId, req.SelectedBankId, req.SelectedAid, req.InstallmentCount)

	// Check if loyalty card inquiry is needed
	if req.UseLoyaltyPoints && txn.Card != nil && txn.Card.HasLoyaltyCard {
		h.txnManager.UpdateTransactionState(req.TransactionId, StateLoyaltyInquiry)

		log.Printf("[HANDLER] Loyalty inquiry - Points: %d, Value: %d cents",
			txn.Card.LoyaltyPoints, txn.Card.PointsValueCents)

		response := &pb.LoyaltyCardInquiryResponse{
			Code:              "00",
			AvailablePoints:   txn.Card.LoyaltyPoints,
			PointsValueCents:  txn.Card.PointsValueCents,
			LoyaltyCardNumber: fmt.Sprintf("LOY-%s", txn.Card.MaskedNumber[len(txn.Card.MaskedNumber)-4:]),
			Message: fmt.Sprintf("You have %d loyalty points available (worth %.2f %s)",
				txn.Card.LoyaltyPoints, float64(txn.Card.PointsValueCents)/100.0, txn.Currency),
		}

		return &pb.Message{
			MessageId: generateMessageID(),
			Timestamp: time.Now().Format(time.RFC3339),
			Body: &pb.Message_LoyaltyCardInquiryResponse{
				LoyaltyCardInquiryResponse: response,
			},
		}, nil
	}

	// No loyalty, proceed directly to payment completion
	return h.processPayment(txn)
}

// handleLoyaltyConfirmation handles loyalty points confirmation
func (h *MessageHandler) handleLoyaltyConfirmation(msg *pb.Message, req *pb.LoyaltyPointsConfirmation) (*pb.Message, error) {
	log.Printf("[HANDLER] Loyalty confirmation - TxnID: %s, Points: %d, Value: %d cents",
		req.TransactionId, req.PointsToUse, req.PointsValueCents)

	txn, err := h.txnManager.GetTransaction(req.TransactionId)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "02", err.Error()), nil
	}

	// Set loyalty points usage
	h.txnManager.SetLoyaltyPoints(req.TransactionId, true, req.PointsToUse, req.PointsValueCents)

	// Process payment with loyalty points
	return h.processPayment(txn)
}

// processPayment processes the actual payment
func (h *MessageHandler) processPayment(txn *Transaction) (*pb.Message, error) {
	h.txnManager.UpdateTransactionState(txn.ID, StatePaymentProcessing)

	// Simulate payment processing delay
	time.Sleep(1 * time.Second)

	// Simulate bank authorization (90% success rate)
	success := rand.Intn(100) < 90

	if !success {
		h.txnManager.FailTransaction(txn.ID, "Payment declined by bank")
		return h.createErrorResponse(txn.ID, "05", "Payment declined by issuer"), nil
	}

	// Generate confirmation codes
	confirmationCode := fmt.Sprintf("CONF-%d", time.Now().Unix())
	authCode := fmt.Sprintf("AUTH-%06d", rand.Intn(1000000))

	// Complete transaction
	h.txnManager.CompleteTransaction(txn.ID, confirmationCode, authCode)

	// Reload transaction to get updated values
	txn, _ = h.txnManager.GetTransaction(txn.ID)
	if txn == nil {
		// Transaction was moved to completed, fetch from manager
		txn = h.txnManager.completedTransactions[len(h.txnManager.completedTransactions)-1]
	}

	cardAmountCents := txn.AmountCents - txn.LoyaltyAmountCents
	remainingPoints := uint32(0)
	if txn.Card != nil && txn.Card.HasLoyaltyCard {
		remainingPoints = txn.Card.LoyaltyPoints - txn.LoyaltyPointsUsed
	}

	log.Printf("[HANDLER] Payment completed - TxnID: %s, Confirmation: %s, Auth: %s",
		txn.ID, confirmationCode, authCode)

	response := &pb.PaymentCompletionResponse{
		Code:               "00",
		ConfirmationCode:   confirmationCode,
		ReceiptNumber:      txn.ReceiptNumber,
		CardAmountCents:    cardAmountCents,
		LoyaltyAmountCents: txn.LoyaltyAmountCents,
		RemainingPoints:    remainingPoints,
		Message:            "Payment completed successfully",
		AuthCode:           authCode,
		Currency:           txn.Currency,
		InstallmentCount:   txn.InstallmentCount,
	}

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_PaymentCompletionResponse{
			PaymentCompletionResponse: response,
		},
	}, nil
}

// handlePaymentCompletion handles payment completion request (alternative flow)
func (h *MessageHandler) handlePaymentCompletion(msg *pb.Message, req *pb.PaymentCompletionRequest) (*pb.Message, error) {
	log.Printf("[HANDLER] Payment completion request - TxnID: %s, Total: %d %s",
		req.TransactionId, req.TotalAmountCents, req.Currency)

	txn, err := h.txnManager.GetTransaction(req.TransactionId)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "02", err.Error()), nil
	}

	return h.processPayment(txn)
}

// handleRefund handles refund transaction request
func (h *MessageHandler) handleRefund(msg *pb.Message, req *pb.RefundTransactionRequest) (*pb.Message, error) {
	log.Printf("[HANDLER] Refund request - OriginalTxnID: %s, Amount: %d %s",
		req.OriginalTransactionId, req.RefundAmountCents, req.Currency)

	_, err := h.txnManager.RefundTransaction(req.OriginalTransactionId, req.RefundAmountCents)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "03", err.Error()), nil
	}

	refundTxnID := fmt.Sprintf("RFD-%s-%d", req.OriginalTransactionId, time.Now().Unix())
	confirmationCode := fmt.Sprintf("RFCONF-%d", time.Now().Unix())

	log.Printf("[HANDLER] Refund completed - RefundTxnID: %s, Confirmation: %s",
		refundTxnID, confirmationCode)

	response := &pb.RefundTransactionResponse{
		Code:              "00",
		TransactionId:     refundTxnID,
		RefundAmountCents: req.RefundAmountCents,
		ConfirmationCode:  confirmationCode,
		Message: fmt.Sprintf("Refund of %.2f %s completed successfully",
			float64(req.RefundAmountCents)/100.0, req.Currency),
		Currency: req.Currency,
	}

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_RefundTransactionResponse{
			RefundTransactionResponse: response,
		},
	}, nil
}

// handleVoid handles void transaction request
func (h *MessageHandler) handleVoid(msg *pb.Message, req *pb.VoidTransactionRequest) (*pb.Message, error) {
	log.Printf("[HANDLER] Void request - OriginalTxnID: %s", req.OriginalTransactionId)

	originalTxn, err := h.txnManager.VoidTransaction(req.OriginalTransactionId)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "04", err.Error()), nil
	}

	voidTxnID := fmt.Sprintf("VOID-%s-%d", req.OriginalTransactionId, time.Now().Unix())
	confirmationCode := fmt.Sprintf("VOIDCONF-%d", time.Now().Unix())

	log.Printf("[HANDLER] Void completed - VoidTxnID: %s, Confirmation: %s",
		voidTxnID, confirmationCode)

	response := &pb.VoidTransactionResponse{
		Code:             "00",
		TransactionId:    voidTxnID,
		ConfirmationCode: confirmationCode,
		Message:          fmt.Sprintf("Transaction %s voided successfully", req.OriginalTransactionId),
		Currency:         originalTxn.Currency,
	}

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_VoidTransactionResponse{
			VoidTransactionResponse: response,
		},
	}, nil
}

// handleXReport handles X report request
func (h *MessageHandler) handleXReport(msg *pb.Message) (*pb.Message, error) {
	log.Printf("[HANDLER] X Report request")

	report := h.txnManager.GenerateXReport()

	log.Printf("[HANDLER] X Report generated - Transactions: %d", report.TransactionCount)

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_XReportResponse{
			XReportResponse: report,
		},
	}, nil
}

// handleZReport handles Z report request
func (h *MessageHandler) handleZReport(msg *pb.Message) (*pb.Message, error) {
	log.Printf("[HANDLER] Z Report request")

	report := h.txnManager.GenerateZReport()

	log.Printf("[HANDLER] Z Report generated - Transactions: %d, Z Number: %d",
		report.TransactionCount, report.ZReportNumber)

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_ZReportResponse{
			ZReportResponse: report,
		},
	}, nil
}

// handleDetailedReport handles detailed report request
func (h *MessageHandler) handleDetailedReport(msg *pb.Message, req *pb.DetailedReportRequest) (*pb.Message, error) {
	log.Printf("[HANDLER] Detailed report request - From: %s, To: %s",
		req.FromTimestamp, req.ToTimestamp)

	fromTime, _ := time.Parse(time.RFC3339, req.FromTimestamp)
	toTime, _ := time.Parse(time.RFC3339, req.ToTimestamp)

	if fromTime.IsZero() {
		fromTime = time.Now().Add(-24 * time.Hour)
	}
	if toTime.IsZero() {
		toTime = time.Now()
	}

	report := h.txnManager.GenerateDetailedReport(fromTime, toTime, req.Limit, req.IncludeVoids)

	log.Printf("[HANDLER] Detailed report generated - Transactions: %d", len(report.Transactions))

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_DetailedReportResponse{
			DetailedReportResponse: report,
		},
	}, nil
}

// handleGiftCardInquiry handles gift card inquiry request
func (h *MessageHandler) handleGiftCardInquiry(msg *pb.Message, req *pb.GiftCardInquiryRequest) (*pb.Message, error) {
	log.Printf("[HANDLER] Gift card inquiry - Card: %s", req.CardNumber)

	// Simulate gift card balance (random between 0 and 50000 cents)
	balance := uint32(rand.Intn(50000))

	response := &pb.GiftCardInquiryResponse{
		Code:         "00",
		BalanceCents: balance,
		Message:      fmt.Sprintf("Gift card balance: %.2f %s", float64(balance)/100.0, req.Currency),
		Currency:     req.Currency,
	}

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GiftCardInquiryResponse{
			GiftCardInquiryResponse: response,
		},
	}, nil
}

// handleGiftCardCharge handles gift card charge request
func (h *MessageHandler) handleGiftCardCharge(msg *pb.Message, req *pb.GiftCardChargeRequest) (*pb.Message, error) {
	log.Printf("[HANDLER] Gift card charge - Card: %s, Amount: %d %s",
		req.CardNumber, req.AmountCents, req.Currency)

	// Simulate gift card charge
	newBalance := uint32(50000) - req.AmountCents // Assuming sufficient balance
	pointsEarned := req.AmountCents / 100         // 1 point per 1 unit of currency

	response := &pb.GiftCardChargeResponse{
		Code:            "00",
		NewBalanceCents: newBalance,
		PointsEarned:    pointsEarned,
		Message: fmt.Sprintf("Gift card charged successfully. New balance: %.2f %s",
			float64(newBalance)/100.0, req.Currency),
		Currency: req.Currency,
	}

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GiftCardChargeResponse{
			GiftCardChargeResponse: response,
		},
	}, nil
}

// createErrorResponse creates an error response message
func (h *MessageHandler) createErrorResponse(messageID, code, errorMsg string) *pb.Message {
	log.Printf("[HANDLER] Error response - Code: %s, Message: %s", code, errorMsg)

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_ErrorResponse{
			ErrorResponse: &pb.ErrorResponse{
				Code:    code,
				Message: errorMsg,
			},
		},
	}
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	return fmt.Sprintf("MSG-%d-%d", time.Now().Unix(), rand.Intn(10000))
}

// GetTransactionManager returns the transaction manager
func (h *MessageHandler) GetTransactionManager() *TransactionManager {
	return h.txnManager
}
