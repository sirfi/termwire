package pos

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	pb "github.com/sirfi/termwire/protocol"
)

// MessageHandler handles incoming protocol messages
type MessageHandler struct {
	config     *Config
	txnManager *TransactionManager
	cardReader CardReader
	logger     *slog.Logger
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(config *Config, logger *slog.Logger) (*MessageHandler, error) {
	tm, err := NewTransactionManager(config)
	if err != nil {
		return nil, fmt.Errorf("creating transaction manager: %w", err)
	}
	return &MessageHandler{
		config:     config,
		txnManager: tm,
		cardReader: NewCardReader(),
		logger:     logger,
	}, nil
}

// Close releases resources held by the handler.
func (h *MessageHandler) Close() error {
	return h.txnManager.Close()
}

// HandleMessage processes an incoming message and returns a response
func (h *MessageHandler) HandleMessage(msg *pb.Message) (*pb.Message, error) {
	h.logger.Info("processing message", slog.String("message_id", msg.MessageId))

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

func (h *MessageHandler) handleGetVersion(msg *pb.Message) (*pb.Message, error) {
	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GetVersionResponse{
			GetVersionResponse: &pb.GetVersionResponse{Code: "00", Version: h.config.Version},
		},
	}, nil
}

func (h *MessageHandler) handleGetTerminalInfo(msg *pb.Message) (*pb.Message, error) {
	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GetTerminalInfoResponse{
			GetTerminalInfoResponse: &pb.GetTerminalInfoResponse{
				Code:         "00",
				Version:      h.config.Version,
				SerialNumber: h.config.SerialNumber,
			},
		},
	}, nil
}

func (h *MessageHandler) handleGetBanks(msg *pb.Message) (*pb.Message, error) {
	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GetBanksResponse{
			GetBanksResponse: &pb.GetBanksResponse{Code: "00", Banks: GetBanks()},
		},
	}, nil
}

func (h *MessageHandler) handleCardInsertion(msg *pb.Message, req *pb.CardInsertionRequest) (*pb.Message, error) {
	h.logger.Info("card insertion request",
		slog.String("transaction_id", req.TransactionId),
		slog.Uint64("amount_cents", uint64(req.TransactionAmountCents)),
		slog.String("currency", req.Currency),
	)

	_, err := h.txnManager.CreateTransaction(req.TransactionId, req.TransactionAmountCents, req.Currency)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "01", err.Error()), nil
	}

	card := h.cardReader.SimulateCardInsertion()
	h.txnManager.SetCardData(req.TransactionId, card)
	h.txnManager.UpdateTransactionState(req.TransactionId, StateBankSelection)

	h.logger.Info("card inserted",
		slog.String("transaction_id", req.TransactionId),
		slog.String("masked_number", card.MaskedNumber),
		slog.Int("bank_count", len(card.AvailableBanks)),
	)

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_CardInsertionResponse{
			CardInsertionResponse: &pb.CardInsertionResponse{
				Code:             "00",
				CardNumberMasked: card.MaskedNumber,
				CardHolderName:   card.HolderName,
				AvailableBanks:   card.AvailableBanks,
				Message:          "Card read successfully. Please select bank.",
			},
		},
	}, nil
}

func (h *MessageHandler) handlePaymentProcessing(msg *pb.Message, req *pb.PaymentProcessingRequest) (*pb.Message, error) {
	h.logger.Info("payment processing",
		slog.String("transaction_id", req.TransactionId),
		slog.Uint64("bank_id", uint64(req.SelectedBankId)),
		slog.Bool("use_loyalty", req.UseLoyaltyPoints),
	)

	// Idempotency: return cached result if already completed.
	if completed := h.txnManager.GetCompletedTransactionByID(req.TransactionId); completed != nil && completed.CachedResponse != nil {
		h.logger.Info("idempotent response", slog.String("transaction_id", req.TransactionId))
		return &pb.Message{
			MessageId: generateMessageID(),
			Timestamp: time.Now().Format(time.RFC3339),
			Body:      &pb.Message_PaymentCompletionResponse{PaymentCompletionResponse: completed.CachedResponse},
		}, nil
	}

	txn, err := h.txnManager.GetTransaction(req.TransactionId)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "02", err.Error()), nil
	}

	h.txnManager.SetBankSelection(req.TransactionId, req.SelectedBankId, req.SelectedAid, req.InstallmentCount)

	if req.UseLoyaltyPoints && txn.Card != nil && txn.Card.HasLoyaltyCard {
		h.txnManager.UpdateTransactionState(req.TransactionId, StateLoyaltyInquiry)

		h.logger.Info("loyalty inquiry",
			slog.String("transaction_id", req.TransactionId),
			slog.Uint64("available_points", uint64(txn.Card.LoyaltyPoints)),
			slog.Uint64("value_cents", uint64(txn.Card.PointsValueCents)),
		)

		return &pb.Message{
			MessageId: generateMessageID(),
			Timestamp: time.Now().Format(time.RFC3339),
			Body: &pb.Message_LoyaltyCardInquiryResponse{
				LoyaltyCardInquiryResponse: &pb.LoyaltyCardInquiryResponse{
					Code:              "00",
					AvailablePoints:   txn.Card.LoyaltyPoints,
					PointsValueCents:  txn.Card.PointsValueCents,
					LoyaltyCardNumber: fmt.Sprintf("LOY-%s", txn.Card.MaskedNumber[len(txn.Card.MaskedNumber)-4:]),
					Message: fmt.Sprintf("You have %d loyalty points available (worth %.2f %s)",
						txn.Card.LoyaltyPoints, float64(txn.Card.PointsValueCents)/100.0, txn.Currency),
				},
			},
		}, nil
	}

	return h.processPayment(txn)
}

func (h *MessageHandler) handleLoyaltyConfirmation(msg *pb.Message, req *pb.LoyaltyPointsConfirmation) (*pb.Message, error) {
	h.logger.Info("loyalty confirmation",
		slog.String("transaction_id", req.TransactionId),
		slog.Uint64("points_to_use", uint64(req.PointsToUse)),
		slog.Uint64("value_cents", uint64(req.PointsValueCents)),
	)

	// Idempotency: return cached result if already completed.
	if completed := h.txnManager.GetCompletedTransactionByID(req.TransactionId); completed != nil && completed.CachedResponse != nil {
		h.logger.Info("idempotent response", slog.String("transaction_id", req.TransactionId))
		return &pb.Message{
			MessageId: generateMessageID(),
			Timestamp: time.Now().Format(time.RFC3339),
			Body:      &pb.Message_PaymentCompletionResponse{PaymentCompletionResponse: completed.CachedResponse},
		}, nil
	}

	txn, err := h.txnManager.GetTransaction(req.TransactionId)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "02", err.Error()), nil
	}

	h.txnManager.SetLoyaltyPoints(req.TransactionId, true, req.PointsToUse, req.PointsValueCents)
	return h.processPayment(txn)
}

func (h *MessageHandler) processPayment(txn *Transaction) (*pb.Message, error) {
	h.txnManager.UpdateTransactionState(txn.ID, StatePaymentProcessing)

	time.Sleep(1 * time.Second)

	success := rand.Intn(100) < 90

	if !success {
		h.txnManager.FailTransaction(txn.ID, "Payment declined by bank")
		return h.createErrorResponse(txn.ID, "05", "Payment declined by issuer"), nil
	}

	confirmationCode := fmt.Sprintf("CONF-%d", time.Now().Unix())
	authCode := fmt.Sprintf("AUTH-%06d", rand.Intn(1000000))

	h.txnManager.CompleteTransaction(txn.ID, confirmationCode, authCode)

	// Transaction was moved to completed after CompleteTransaction; retrieve it safely.
	txn = h.txnManager.GetCompletedTransactionByID(txn.ID)

	cardAmountCents := txn.AmountCents - txn.LoyaltyAmountCents
	remainingPoints := uint32(0)
	if txn.Card != nil && txn.Card.HasLoyaltyCard {
		remainingPoints = txn.Card.LoyaltyPoints - txn.LoyaltyPointsUsed
	}

	h.logger.Info("payment completed",
		slog.String("transaction_id", txn.ID),
		slog.String("confirmation_code", confirmationCode),
		slog.String("auth_code", authCode),
		slog.String("receipt_number", txn.ReceiptNumber),
	)

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

	// Cache for idempotent retries.
	txn.CachedResponse = response
	h.txnManager.SetCachedResponse(txn.ID, response)

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_PaymentCompletionResponse{
			PaymentCompletionResponse: response,
		},
	}, nil
}

func (h *MessageHandler) handlePaymentCompletion(msg *pb.Message, req *pb.PaymentCompletionRequest) (*pb.Message, error) {
	h.logger.Info("payment completion request",
		slog.String("transaction_id", req.TransactionId),
		slog.Uint64("total_cents", uint64(req.TotalAmountCents)),
		slog.String("currency", req.Currency),
	)

	txn, err := h.txnManager.GetTransaction(req.TransactionId)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "02", err.Error()), nil
	}

	return h.processPayment(txn)
}

func (h *MessageHandler) handleRefund(msg *pb.Message, req *pb.RefundTransactionRequest) (*pb.Message, error) {
	h.logger.Info("refund request",
		slog.String("original_transaction_id", req.OriginalTransactionId),
		slog.Uint64("refund_amount_cents", uint64(req.RefundAmountCents)),
		slog.String("currency", req.Currency),
	)

	_, err := h.txnManager.RefundTransaction(req.OriginalTransactionId, req.RefundAmountCents)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "03", err.Error()), nil
	}

	refundTxnID := fmt.Sprintf("RFD-%s-%d", req.OriginalTransactionId, time.Now().Unix())
	confirmationCode := fmt.Sprintf("RFCONF-%d", time.Now().Unix())

	h.logger.Info("refund completed",
		slog.String("refund_transaction_id", refundTxnID),
		slog.String("confirmation_code", confirmationCode),
	)

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_RefundTransactionResponse{
			RefundTransactionResponse: &pb.RefundTransactionResponse{
				Code:              "00",
				TransactionId:     refundTxnID,
				RefundAmountCents: req.RefundAmountCents,
				ConfirmationCode:  confirmationCode,
				Message: fmt.Sprintf("Refund of %.2f %s completed successfully",
					float64(req.RefundAmountCents)/100.0, req.Currency),
				Currency: req.Currency,
			},
		},
	}, nil
}

func (h *MessageHandler) handleVoid(msg *pb.Message, req *pb.VoidTransactionRequest) (*pb.Message, error) {
	h.logger.Info("void request", slog.String("original_transaction_id", req.OriginalTransactionId))

	originalTxn, err := h.txnManager.VoidTransaction(req.OriginalTransactionId)
	if err != nil {
		return h.createErrorResponse(msg.MessageId, "04", err.Error()), nil
	}

	voidTxnID := fmt.Sprintf("VOID-%s-%d", req.OriginalTransactionId, time.Now().Unix())
	confirmationCode := fmt.Sprintf("VOIDCONF-%d", time.Now().Unix())

	h.logger.Info("void completed",
		slog.String("void_transaction_id", voidTxnID),
		slog.String("confirmation_code", confirmationCode),
	)

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_VoidTransactionResponse{
			VoidTransactionResponse: &pb.VoidTransactionResponse{
				Code:             "00",
				TransactionId:    voidTxnID,
				ConfirmationCode: confirmationCode,
				Message:          fmt.Sprintf("Transaction %s voided successfully", req.OriginalTransactionId),
				Currency:         originalTxn.Currency,
			},
		},
	}, nil
}

func (h *MessageHandler) handleXReport(msg *pb.Message) (*pb.Message, error) {
	report := h.txnManager.GenerateXReport()
	h.logger.Info("X report generated", slog.Uint64("transaction_count", uint64(report.TransactionCount)))
	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body:      &pb.Message_XReportResponse{XReportResponse: report},
	}, nil
}

func (h *MessageHandler) handleZReport(msg *pb.Message) (*pb.Message, error) {
	report := h.txnManager.GenerateZReport()
	h.logger.Info("Z report generated",
		slog.Uint64("transaction_count", uint64(report.TransactionCount)),
		slog.Uint64("z_report_number", uint64(report.ZReportNumber)),
	)
	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body:      &pb.Message_ZReportResponse{ZReportResponse: report},
	}, nil
}

func (h *MessageHandler) handleDetailedReport(msg *pb.Message, req *pb.DetailedReportRequest) (*pb.Message, error) {
	h.logger.Info("detailed report request",
		slog.String("from", req.FromTimestamp),
		slog.String("to", req.ToTimestamp),
	)

	fromTime, _ := time.Parse(time.RFC3339, req.FromTimestamp)
	toTime, _ := time.Parse(time.RFC3339, req.ToTimestamp)

	if fromTime.IsZero() {
		fromTime = time.Now().Add(-24 * time.Hour)
	}
	if toTime.IsZero() {
		toTime = time.Now()
	}

	report := h.txnManager.GenerateDetailedReport(fromTime, toTime, req.Limit, req.IncludeVoids)
	h.logger.Info("detailed report generated", slog.Int("transaction_count", len(report.Transactions)))

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body:      &pb.Message_DetailedReportResponse{DetailedReportResponse: report},
	}, nil
}

func (h *MessageHandler) handleGiftCardInquiry(msg *pb.Message, req *pb.GiftCardInquiryRequest) (*pb.Message, error) {
	h.logger.Info("gift card inquiry", slog.String("card_number", req.CardNumber))

	balance := uint32(rand.Intn(50000))

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GiftCardInquiryResponse{
			GiftCardInquiryResponse: &pb.GiftCardInquiryResponse{
				Code:         "00",
				BalanceCents: balance,
				Message:      fmt.Sprintf("Gift card balance: %.2f %s", float64(balance)/100.0, req.Currency),
				Currency:     req.Currency,
			},
		},
	}, nil
}

func (h *MessageHandler) handleGiftCardCharge(msg *pb.Message, req *pb.GiftCardChargeRequest) (*pb.Message, error) {
	h.logger.Info("gift card charge",
		slog.String("card_number", req.CardNumber),
		slog.Uint64("amount_cents", uint64(req.AmountCents)),
		slog.String("currency", req.Currency),
	)

	newBalance := uint32(50000) - req.AmountCents
	pointsEarned := req.AmountCents / 100

	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GiftCardChargeResponse{
			GiftCardChargeResponse: &pb.GiftCardChargeResponse{
				Code:            "00",
				NewBalanceCents: newBalance,
				PointsEarned:    pointsEarned,
				Message: fmt.Sprintf("Gift card charged successfully. New balance: %.2f %s",
					float64(newBalance)/100.0, req.Currency),
				Currency: req.Currency,
			},
		},
	}, nil
}

func (h *MessageHandler) createErrorResponse(messageID, code, errorMsg string) *pb.Message {
	h.logger.Error("error response", slog.String("code", code), slog.String("message", errorMsg))
	return &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_ErrorResponse{
			ErrorResponse: &pb.ErrorResponse{Code: code, Message: errorMsg},
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
