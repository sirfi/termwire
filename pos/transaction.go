package pos

import (
	"fmt"
	"sync"
	"time"

	pb "github.com/sirfi/termwire/protocol"
)

// TransactionState represents the current state of a transaction
type TransactionState int

const (
	StateCardInsertion TransactionState = iota
	StateBankSelection
	StateLoyaltyInquiry
	StateLoyaltyConfirmation
	StatePaymentProcessing
	StatePaymentCompleted
	StateFailed
	StateVoided
	StateRefunded
)

func (s TransactionState) String() string {
	switch s {
	case StateCardInsertion:
		return "CARD_INSERTION"
	case StateBankSelection:
		return "BANK_SELECTION"
	case StateLoyaltyInquiry:
		return "LOYALTY_INQUIRY"
	case StateLoyaltyConfirmation:
		return "LOYALTY_CONFIRMATION"
	case StatePaymentProcessing:
		return "PAYMENT_PROCESSING"
	case StatePaymentCompleted:
		return "PAYMENT_COMPLETED"
	case StateFailed:
		return "FAILED"
	case StateVoided:
		return "VOIDED"
	case StateRefunded:
		return "REFUNDED"
	default:
		return "UNKNOWN"
	}
}

// Transaction represents a single payment transaction
type Transaction struct {
	ID                 string
	State              TransactionState
	AmountCents        uint32
	Currency           string
	Card               *CardData
	SelectedBankID     uint32
	SelectedAID        string
	InstallmentCount   uint32
	UseLoyaltyPoints   bool
	LoyaltyAmountCents uint32
	LoyaltyPointsUsed  uint32
	CardAmountCents    uint32
	ConfirmationCode   string
	ReceiptNumber      string
	AuthCode           string
	ErrorMessage       string
	CreatedAt          time.Time
	CompletedAt        *time.Time
	LastUpdated        time.Time
}

// TransactionManager manages active and historical transactions
type TransactionManager struct {
	mu                    sync.RWMutex
	activeTransactions    map[string]*Transaction
	completedTransactions []*Transaction
	receiptCounter        uint64
	config                *Config
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(config *Config) *TransactionManager {
	return &TransactionManager{
		activeTransactions:    make(map[string]*Transaction),
		completedTransactions: make([]*Transaction, 0),
		receiptCounter:        1000,
		config:                config,
	}
}

// CreateTransaction creates a new transaction
func (tm *TransactionManager) CreateTransaction(transactionID string, amountCents uint32, currency string) (*Transaction, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if transaction already exists
	if _, exists := tm.activeTransactions[transactionID]; exists {
		return nil, fmt.Errorf("transaction %s already exists", transactionID)
	}

	// Validate amount
	if amountCents == 0 {
		return nil, fmt.Errorf("invalid transaction amount: 0")
	}

	if amountCents > uint32(tm.config.MaxTransactionAmount) {
		return nil, fmt.Errorf("transaction amount %d exceeds maximum %d", amountCents, tm.config.MaxTransactionAmount)
	}

	// Validate currency
	validCurrency := false
	for _, curr := range tm.config.SupportedCurrencies {
		if curr == currency {
			validCurrency = true
			break
		}
	}
	if !validCurrency {
		return nil, fmt.Errorf("unsupported currency: %s", currency)
	}

	txn := &Transaction{
		ID:          transactionID,
		State:       StateCardInsertion,
		AmountCents: amountCents,
		Currency:    currency,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
	}

	tm.activeTransactions[transactionID] = txn
	return txn, nil
}

// GetTransaction retrieves an active transaction
func (tm *TransactionManager) GetTransaction(transactionID string) (*Transaction, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	txn, exists := tm.activeTransactions[transactionID]
	if !exists {
		return nil, fmt.Errorf("transaction %s not found", transactionID)
	}

	return txn, nil
}

// UpdateTransactionState updates the state of a transaction
func (tm *TransactionManager) UpdateTransactionState(transactionID string, state TransactionState) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	txn, exists := tm.activeTransactions[transactionID]
	if !exists {
		return fmt.Errorf("transaction %s not found", transactionID)
	}

	txn.State = state
	txn.LastUpdated = time.Now()
	return nil
}

// SetCardData sets the card data for a transaction
func (tm *TransactionManager) SetCardData(transactionID string, card *CardData) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	txn, exists := tm.activeTransactions[transactionID]
	if !exists {
		return fmt.Errorf("transaction %s not found", transactionID)
	}

	txn.Card = card
	txn.LastUpdated = time.Now()
	return nil
}

// SetBankSelection sets the selected bank for a transaction
func (tm *TransactionManager) SetBankSelection(transactionID string, bankID uint32, aid string, installments uint32) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	txn, exists := tm.activeTransactions[transactionID]
	if !exists {
		return fmt.Errorf("transaction %s not found", transactionID)
	}

	txn.SelectedBankID = bankID
	txn.SelectedAID = aid
	txn.InstallmentCount = installments
	txn.LastUpdated = time.Now()
	return nil
}

// SetLoyaltyPoints sets loyalty points usage for a transaction
func (tm *TransactionManager) SetLoyaltyPoints(transactionID string, useLoyalty bool, pointsUsed, amountCents uint32) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	txn, exists := tm.activeTransactions[transactionID]
	if !exists {
		return fmt.Errorf("transaction %s not found", transactionID)
	}

	txn.UseLoyaltyPoints = useLoyalty
	txn.LoyaltyPointsUsed = pointsUsed
	txn.LoyaltyAmountCents = amountCents
	txn.LastUpdated = time.Now()
	return nil
}

// CompleteTransaction completes a transaction and moves it to history
func (tm *TransactionManager) CompleteTransaction(transactionID, confirmationCode, authCode string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	txn, exists := tm.activeTransactions[transactionID]
	if !exists {
		return fmt.Errorf("transaction %s not found", transactionID)
	}

	now := time.Now()
	txn.State = StatePaymentCompleted
	txn.ConfirmationCode = confirmationCode
	txn.AuthCode = authCode
	tm.receiptCounter++
	txn.ReceiptNumber = fmt.Sprintf("RCP-%d", tm.receiptCounter)
	txn.CompletedAt = &now
	txn.LastUpdated = now

	// Calculate card amount
	txn.CardAmountCents = txn.AmountCents - txn.LoyaltyAmountCents

	// Move to completed transactions
	tm.completedTransactions = append(tm.completedTransactions, txn)
	delete(tm.activeTransactions, transactionID)

	return nil
}

// FailTransaction marks a transaction as failed
func (tm *TransactionManager) FailTransaction(transactionID, errorMsg string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	txn, exists := tm.activeTransactions[transactionID]
	if !exists {
		return fmt.Errorf("transaction %s not found", transactionID)
	}

	now := time.Now()
	txn.State = StateFailed
	txn.ErrorMessage = errorMsg
	txn.CompletedAt = &now
	txn.LastUpdated = now

	// Move to completed transactions
	tm.completedTransactions = append(tm.completedTransactions, txn)
	delete(tm.activeTransactions, transactionID)

	return nil
}

// VoidTransaction voids a completed transaction
func (tm *TransactionManager) VoidTransaction(originalTxnID string) (*Transaction, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Find the original transaction in completed transactions
	var originalTxn *Transaction
	for _, txn := range tm.completedTransactions {
		if txn.ID == originalTxnID && txn.State == StatePaymentCompleted {
			originalTxn = txn
			break
		}
	}

	if originalTxn == nil {
		return nil, fmt.Errorf("completed transaction %s not found", originalTxnID)
	}

	// Update state to voided
	now := time.Now()
	originalTxn.State = StateVoided
	originalTxn.LastUpdated = now

	return originalTxn, nil
}

// RefundTransaction creates a refund for a completed transaction
func (tm *TransactionManager) RefundTransaction(originalTxnID string, refundAmount uint32) (*Transaction, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Find the original transaction in completed transactions
	var originalTxn *Transaction
	for _, txn := range tm.completedTransactions {
		if txn.ID == originalTxnID && txn.State == StatePaymentCompleted {
			originalTxn = txn
			break
		}
	}

	if originalTxn == nil {
		return nil, fmt.Errorf("completed transaction %s not found", originalTxnID)
	}

	// Validate refund amount
	if refundAmount > originalTxn.AmountCents {
		return nil, fmt.Errorf("refund amount %d exceeds original transaction amount %d", refundAmount, originalTxn.AmountCents)
	}

	// Update state to refunded
	now := time.Now()
	originalTxn.State = StateRefunded
	originalTxn.LastUpdated = now

	return originalTxn, nil
}

// GetActiveTransactionCount returns the count of active transactions
func (tm *TransactionManager) GetActiveTransactionCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.activeTransactions)
}

// GetCompletedTransactionCount returns the count of completed transactions
func (tm *TransactionManager) GetCompletedTransactionCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.completedTransactions)
}

// GetStatistics returns transaction statistics
func (tm *TransactionManager) GetStatistics() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["active_count"] = len(tm.activeTransactions)
	stats["completed_count"] = len(tm.completedTransactions)

	// Calculate totals by state
	stateCounts := make(map[string]int)
	var totalAmount, completedAmount, refundedAmount uint32

	for _, txn := range tm.completedTransactions {
		stateCounts[txn.State.String()]++
		totalAmount += txn.AmountCents

		if txn.State == StatePaymentCompleted {
			completedAmount += txn.AmountCents
		} else if txn.State == StateRefunded {
			refundedAmount += txn.AmountCents
		}
	}

	stats["state_counts"] = stateCounts
	stats["total_amount_cents"] = totalAmount
	stats["completed_amount_cents"] = completedAmount
	stats["refunded_amount_cents"] = refundedAmount

	return stats
}

// GetTransactionsByDateRange returns transactions within a date range
func (tm *TransactionManager) GetTransactionsByDateRange(from, to time.Time) []*Transaction {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var transactions []*Transaction
	for _, txn := range tm.completedTransactions {
		if txn.CreatedAt.After(from) && txn.CreatedAt.Before(to) {
			transactions = append(transactions, txn)
		}
	}

	return transactions
}

// GenerateXReport generates an X report (daily totals without resetting)
func (tm *TransactionManager) GenerateXReport() *pb.XReportResponse {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.generateReport(false)
}

// GenerateZReport generates a Z report (daily totals with reset)
func (tm *TransactionManager) GenerateZReport() *pb.ZReportResponse {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	report := tm.generateReport(true)

	// Convert to Z report format
	zReport := &pb.ZReportResponse{
		Code:                report.Code,
		ReportTimestamp:     report.ReportTimestamp,
		TransactionCount:    report.TransactionCount,
		SalesTotals:         report.SalesTotals,
		RefundTotals:        report.RefundTotals,
		VoidTotals:          report.VoidTotals,
		PaymentMethodTotals: report.PaymentMethodTotals,
		ZReportNumber:       uint32(time.Now().Unix()), // Simple Z report number
		Message:             "Z Report generated successfully",
	}

	// Reset completed transactions after Z report
	tm.completedTransactions = make([]*Transaction, 0)

	return zReport
}

// generateReport is a helper function to generate X/Z reports
func (tm *TransactionManager) generateReport(isZReport bool) *pb.XReportResponse {
	salesTotals := make(map[string]*pb.CurrencyTotals)
	refundTotals := make(map[string]*pb.CurrencyTotals)
	voidTotals := make(map[string]*pb.CurrencyTotals)

	var transactionCount uint32

	for _, txn := range tm.completedTransactions {
		if txn.State == StatePaymentCompleted {
			transactionCount++
			if _, exists := salesTotals[txn.Currency]; !exists {
				salesTotals[txn.Currency] = &pb.CurrencyTotals{
					Currency:    txn.Currency,
					AmountCents: 0,
					Count:       0,
				}
			}
			salesTotals[txn.Currency].AmountCents += txn.AmountCents
			salesTotals[txn.Currency].Count++
		} else if txn.State == StateRefunded {
			if _, exists := refundTotals[txn.Currency]; !exists {
				refundTotals[txn.Currency] = &pb.CurrencyTotals{
					Currency:    txn.Currency,
					AmountCents: 0,
					Count:       0,
				}
			}
			refundTotals[txn.Currency].AmountCents += txn.AmountCents
			refundTotals[txn.Currency].Count++
		} else if txn.State == StateVoided {
			if _, exists := voidTotals[txn.Currency]; !exists {
				voidTotals[txn.Currency] = &pb.CurrencyTotals{
					Currency:    txn.Currency,
					AmountCents: 0,
					Count:       0,
				}
			}
			voidTotals[txn.Currency].AmountCents += txn.AmountCents
			voidTotals[txn.Currency].Count++
		}
	}

	// Convert maps to slices
	sales := make([]*pb.CurrencyTotals, 0, len(salesTotals))
	for _, total := range salesTotals {
		sales = append(sales, total)
	}

	refunds := make([]*pb.CurrencyTotals, 0, len(refundTotals))
	for _, total := range refundTotals {
		refunds = append(refunds, total)
	}

	voids := make([]*pb.CurrencyTotals, 0, len(voidTotals))
	for _, total := range voidTotals {
		voids = append(voids, total)
	}

	return &pb.XReportResponse{
		Code:             "00",
		ReportTimestamp:  time.Now().Format(time.RFC3339),
		TransactionCount: transactionCount,
		SalesTotals:      sales,
		RefundTotals:     refunds,
		VoidTotals:       voids,
		Message:          "Report generated successfully",
	}
}

// GenerateDetailedReport generates a detailed transaction report
func (tm *TransactionManager) GenerateDetailedReport(fromTime, toTime time.Time, limit uint32, includeVoids bool) *pb.DetailedReportResponse {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var entries []*pb.TransactionEntry

	for _, txn := range tm.completedTransactions {
		// Filter by time range
		if txn.CreatedAt.Before(fromTime) || txn.CreatedAt.After(toTime) {
			continue
		}

		// Filter voids if requested
		if !includeVoids && txn.State == StateVoided {
			continue
		}

		// Apply limit
		if limit > 0 && uint32(len(entries)) >= limit {
			break
		}

		var txnType pb.TransactionType
		switch txn.State {
		case StatePaymentCompleted:
			txnType = pb.TransactionType_SALE
		case StateRefunded:
			txnType = pb.TransactionType_REFUND
		case StateVoided:
			txnType = pb.TransactionType_VOID
		default:
			continue
		}

		cardLastFour := ""
		if txn.Card != nil && len(txn.Card.CardNumber) >= 4 {
			cardLastFour = txn.Card.CardNumber[len(txn.Card.CardNumber)-4:]
		}

		entry := &pb.TransactionEntry{
			TransactionId:      txn.ID,
			Type:               txnType,
			AmountCents:        txn.AmountCents,
			PaymentMethod:      pb.PaymentMethod_CREDIT_CARD,
			CardLastFour:       cardLastFour,
			LoyaltyAmountCents: txn.LoyaltyAmountCents,
			ConfirmationCode:   txn.ConfirmationCode,
			ReceiptNumber:      txn.ReceiptNumber,
			Timestamp:          txn.CreatedAt.Format(time.RFC3339),
			BankId:             txn.SelectedBankID,
			Currency:           txn.Currency,
			InstallmentCount:   txn.InstallmentCount,
		}

		entries = append(entries, entry)
	}

	return &pb.DetailedReportResponse{
		Code:         "00",
		Transactions: entries,
		Message:      "Detailed report generated successfully",
	}
}
