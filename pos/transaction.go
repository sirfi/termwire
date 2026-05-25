package pos

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	pb "github.com/sirfi/termwire/protocol"
	"google.golang.org/protobuf/proto"
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

	// CachedResponse holds the serialized PaymentCompletionResponse for idempotent retries.
	CachedResponse *pb.PaymentCompletionResponse
}

// TransactionManager manages active and historical transactions
type TransactionManager struct {
	mu                 sync.RWMutex
	activeTransactions map[string]*Transaction
	receiptCounter     uint64
	config             *Config
	db                 *sql.DB
}

// NewTransactionManager creates a new transaction manager backed by SQLite.
func NewTransactionManager(config *Config) (*TransactionManager, error) {
	db, err := openDB(config.DBFile)
	if err != nil {
		return nil, fmt.Errorf("opening transaction DB: %w", err)
	}

	tm := &TransactionManager{
		activeTransactions: make(map[string]*Transaction),
		receiptCounter:     1000,
		config:             config,
		db:                 db,
	}

	// Load persisted receipt counter
	var val string
	if err := db.QueryRow(`SELECT value FROM metadata WHERE key='receipt_counter'`).Scan(&val); err == nil {
		var n uint64
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
			tm.receiptCounter = n
		}
	}

	return tm, nil
}

// Close releases the underlying database connection.
func (tm *TransactionManager) Close() error {
	return tm.db.Close()
}

// CreateTransaction creates a new transaction
func (tm *TransactionManager) CreateTransaction(transactionID string, amountCents uint32, currency string) (*Transaction, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.activeTransactions[transactionID]; exists {
		return nil, fmt.Errorf("transaction %s already exists", transactionID)
	}

	if amountCents == 0 {
		return nil, fmt.Errorf("invalid transaction amount: 0")
	}

	if amountCents > uint32(tm.config.MaxTransactionAmount) {
		return nil, fmt.Errorf("transaction amount %d exceeds maximum %d", amountCents, tm.config.MaxTransactionAmount)
	}

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

// GetCompletedTransactionByID retrieves a completed transaction by ID from the database.
func (tm *TransactionManager) GetCompletedTransactionByID(transactionID string) *Transaction {
	row := tm.db.QueryRow(selectColumns+` FROM transactions WHERE id = ?`, transactionID)
	return scanTransaction(row)
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

// SetCachedResponse persists the cached payment response for idempotent retries.
func (tm *TransactionManager) SetCachedResponse(transactionID string, resp *pb.PaymentCompletionResponse) error {
	data, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling cached response: %w", err)
	}
	_, err = tm.db.Exec(`UPDATE transactions SET cached_response = ? WHERE id = ?`, data, transactionID)
	return err
}

// CompleteTransaction completes a transaction and persists it to the database.
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
	txn.CardAmountCents = txn.AmountCents - txn.LoyaltyAmountCents

	if err := insertCompleted(tm.db, txn); err != nil {
		return fmt.Errorf("persisting transaction: %w", err)
	}
	tm.db.Exec(`INSERT OR REPLACE INTO metadata (key, value) VALUES ('receipt_counter', ?)`,
		fmt.Sprintf("%d", tm.receiptCounter))

	delete(tm.activeTransactions, transactionID)
	return nil
}

// FailTransaction marks a transaction as failed and persists it to the database.
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

	if err := insertCompleted(tm.db, txn); err != nil {
		return fmt.Errorf("persisting failed transaction: %w", err)
	}
	delete(tm.activeTransactions, transactionID)
	return nil
}

// VoidTransaction voids a completed transaction.
func (tm *TransactionManager) VoidTransaction(originalTxnID string) (*Transaction, error) {
	now := time.Now()
	result, err := tm.db.Exec(`
		UPDATE transactions SET state = ?, last_updated = ?
		WHERE id = ? AND state = ?`,
		int(StateVoided), now.Format(time.RFC3339),
		originalTxnID, int(StatePaymentCompleted))
	if err != nil {
		return nil, fmt.Errorf("voiding transaction: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("completed transaction %s not found", originalTxnID)
	}

	txn := tm.GetCompletedTransactionByID(originalTxnID)
	if txn == nil {
		return nil, fmt.Errorf("failed to retrieve voided transaction %s", originalTxnID)
	}
	return txn, nil
}

// RefundTransaction creates a refund for a completed transaction.
func (tm *TransactionManager) RefundTransaction(originalTxnID string, refundAmount uint32) (*Transaction, error) {
	var amountCents uint32
	var state int
	err := tm.db.QueryRow(`SELECT amount_cents, state FROM transactions WHERE id = ?`, originalTxnID).
		Scan(&amountCents, &state)
	if err != nil || TransactionState(state) != StatePaymentCompleted {
		return nil, fmt.Errorf("completed transaction %s not found", originalTxnID)
	}

	if refundAmount > amountCents {
		return nil, fmt.Errorf("refund amount %d exceeds original transaction amount %d", refundAmount, amountCents)
	}

	now := time.Now()
	if _, err = tm.db.Exec(`UPDATE transactions SET state = ?, last_updated = ? WHERE id = ?`,
		int(StateRefunded), now.Format(time.RFC3339), originalTxnID); err != nil {
		return nil, fmt.Errorf("refunding transaction: %w", err)
	}

	txn := tm.GetCompletedTransactionByID(originalTxnID)
	if txn == nil {
		return nil, fmt.Errorf("failed to retrieve refunded transaction %s", originalTxnID)
	}
	return txn, nil
}

// GetActiveTransactionCount returns the count of active transactions
func (tm *TransactionManager) GetActiveTransactionCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.activeTransactions)
}

// GetCompletedTransactionCount returns the total count of persisted transactions.
func (tm *TransactionManager) GetCompletedTransactionCount() int {
	var count int
	tm.db.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&count)
	return count
}

// GetStatistics returns transaction statistics
func (tm *TransactionManager) GetStatistics() map[string]interface{} {
	tm.mu.RLock()
	activeCount := len(tm.activeTransactions)
	tm.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["active_count"] = activeCount

	var completedCount int
	tm.db.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&completedCount)
	stats["completed_count"] = completedCount

	rows, err := tm.db.Query(`SELECT state, SUM(amount_cents), COUNT(*) FROM transactions GROUP BY state`)
	if err != nil {
		return stats
	}
	defer rows.Close()

	stateCounts := make(map[string]int)
	var totalAmount, completedAmount, refundedAmount uint64
	for rows.Next() {
		var st, cnt int
		var total uint64
		if err := rows.Scan(&st, &total, &cnt); err != nil {
			continue
		}
		s := TransactionState(st)
		stateCounts[s.String()] = cnt
		totalAmount += total
		if s == StatePaymentCompleted {
			completedAmount = total
		} else if s == StateRefunded {
			refundedAmount = total
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
	rows, err := tm.db.Query(
		selectColumns+` FROM transactions WHERE created_at >= ? AND created_at <= ?`,
		from.Format(time.RFC3339), to.Format(time.RFC3339))
	if err != nil {
		return nil
	}
	defer rows.Close()

	var transactions []*Transaction
	for rows.Next() {
		if txn := scanTransaction(rows); txn != nil {
			transactions = append(transactions, txn)
		}
	}
	return transactions
}

// GenerateXReport generates an X report (totals for the current batch, no reset)
func (tm *TransactionManager) GenerateXReport() *pb.XReportResponse {
	return tm.generateReport()
}

// GenerateZReport generates a Z report (totals for current batch, closes batch)
func (tm *TransactionManager) GenerateZReport() *pb.ZReportResponse {
	// Hold the mutex so no CompleteTransaction can sneak in between the SELECT and UPDATE.
	tm.mu.Lock()
	defer tm.mu.Unlock()

	report := tm.generateReport()

	var maxID int
	tm.db.QueryRow(`SELECT COALESCE(MAX(z_report_id), 0) FROM transactions`).Scan(&maxID)
	nextID := maxID + 1
	tm.db.Exec(`UPDATE transactions SET z_report_id = ? WHERE z_report_id = 0`, nextID)

	return &pb.ZReportResponse{
		Code:                report.Code,
		ReportTimestamp:     report.ReportTimestamp,
		TransactionCount:    report.TransactionCount,
		SalesTotals:         report.SalesTotals,
		RefundTotals:        report.RefundTotals,
		VoidTotals:          report.VoidTotals,
		PaymentMethodTotals: report.PaymentMethodTotals,
		ZReportNumber:       uint32(nextID),
		Message:             "Z Report generated successfully",
	}
}

// generateReport builds report totals for the current batch (z_report_id = 0).
func (tm *TransactionManager) generateReport() *pb.XReportResponse {
	rows, err := tm.db.Query(`
		SELECT currency, state, SUM(amount_cents), COUNT(*)
		FROM transactions
		WHERE z_report_id = 0
		GROUP BY currency, state`)

	salesTotals := make(map[string]*pb.CurrencyTotals)
	refundTotals := make(map[string]*pb.CurrencyTotals)
	voidTotals := make(map[string]*pb.CurrencyTotals)
	var transactionCount uint32

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var currency string
			var state int
			var total uint64
			var count uint32
			if err := rows.Scan(&currency, &state, &total, &count); err != nil {
				continue
			}
			ct := &pb.CurrencyTotals{Currency: currency, AmountCents: uint32(total), Count: count}
			switch TransactionState(state) {
			case StatePaymentCompleted:
				transactionCount += count
				salesTotals[currency] = ct
			case StateRefunded:
				refundTotals[currency] = ct
			case StateVoided:
				voidTotals[currency] = ct
			}
		}
	}

	toSlice := func(m map[string]*pb.CurrencyTotals) []*pb.CurrencyTotals {
		s := make([]*pb.CurrencyTotals, 0, len(m))
		for _, v := range m {
			s = append(s, v)
		}
		return s
	}

	return &pb.XReportResponse{
		Code:             "00",
		ReportTimestamp:  time.Now().Format(time.RFC3339),
		TransactionCount: transactionCount,
		SalesTotals:      toSlice(salesTotals),
		RefundTotals:     toSlice(refundTotals),
		VoidTotals:       toSlice(voidTotals),
		Message:          "Report generated successfully",
	}
}

// GenerateDetailedReport generates a detailed transaction report
func (tm *TransactionManager) GenerateDetailedReport(fromTime, toTime time.Time, limit uint32, includeVoids bool) *pb.DetailedReportResponse {
	query := selectColumns + ` FROM transactions WHERE created_at >= ? AND created_at <= ?`
	if !includeVoids {
		query += fmt.Sprintf(` AND state != %d`, int(StateVoided))
	}
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := tm.db.Query(query, fromTime.Format(time.RFC3339), toTime.Format(time.RFC3339))
	if err != nil {
		return &pb.DetailedReportResponse{Code: "00", Transactions: nil, Message: "Detailed report generated successfully"}
	}
	defer rows.Close()

	var entries []*pb.TransactionEntry
	for rows.Next() {
		txn := scanTransaction(rows)
		if txn == nil {
			continue
		}

		var txnType pb.TransactionType
		switch txn.State {
		case StatePaymentCompleted:
			txnType = pb.TransactionType_TRANSACTION_TYPE_SALE
		case StateRefunded:
			txnType = pb.TransactionType_TRANSACTION_TYPE_REFUND
		case StateVoided:
			txnType = pb.TransactionType_TRANSACTION_TYPE_VOID
		default:
			continue
		}

		// CardNumber is stored as the last-four digits; slice is safe since len >= 4.
		cardLastFour := ""
		if txn.Card != nil && len(txn.Card.CardNumber) >= 4 {
			cardLastFour = txn.Card.CardNumber[len(txn.Card.CardNumber)-4:]
		}

		entries = append(entries, &pb.TransactionEntry{
			TransactionId:      txn.ID,
			Type:               txnType,
			AmountCents:        txn.AmountCents,
			PaymentMethod:      pb.PaymentMethod_PAYMENT_METHOD_CREDIT_CARD,
			CardLastFour:       cardLastFour,
			LoyaltyAmountCents: txn.LoyaltyAmountCents,
			ConfirmationCode:   txn.ConfirmationCode,
			ReceiptNumber:      txn.ReceiptNumber,
			Timestamp:          txn.CreatedAt.Format(time.RFC3339),
			BankId:             txn.SelectedBankID,
			Currency:           txn.Currency,
			InstallmentCount:   txn.InstallmentCount,
		})
	}

	return &pb.DetailedReportResponse{
		Code:         "00",
		Transactions: entries,
		Message:      "Detailed report generated successfully",
	}
}

// scanner is implemented by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

const selectColumns = `SELECT
	id, state, amount_cents, currency,
	card_holder_name, card_last_four, card_has_loyalty, card_loyalty_points,
	selected_bank_id, selected_aid, installments,
	use_loyalty_points, loyalty_points_used, loyalty_amount_cents, card_amount_cents,
	confirmation_code, receipt_number, auth_code, error_message,
	created_at, completed_at, last_updated, cached_response`

func scanTransaction(s scanner) *Transaction {
	var (
		txn            Transaction
		state          int
		cardHasLoyalty int
		cardLoyaltyPts int
		useLoyalty     int
		completedAt    string
		cachedBlob     []byte
		cardHolderName string
		cardLastFour   string
		createdAtStr   string
		lastUpdatedStr string
	)

	if err := s.Scan(
		&txn.ID, &state, &txn.AmountCents, &txn.Currency,
		&cardHolderName, &cardLastFour, &cardHasLoyalty, &cardLoyaltyPts,
		&txn.SelectedBankID, &txn.SelectedAID, &txn.InstallmentCount,
		&useLoyalty, &txn.LoyaltyPointsUsed, &txn.LoyaltyAmountCents, &txn.CardAmountCents,
		&txn.ConfirmationCode, &txn.ReceiptNumber, &txn.AuthCode, &txn.ErrorMessage,
		&createdAtStr, &completedAt, &lastUpdatedStr, &cachedBlob,
	); err != nil {
		return nil
	}

	txn.State = TransactionState(state)
	txn.UseLoyaltyPoints = useLoyalty != 0

	// Reconstruct Card with fields persisted at completion time.
	// CardNumber is set to the last-four digits so GenerateDetailedReport can derive it.
	txn.Card = &CardData{
		CardNumber:     cardLastFour,
		HolderName:     cardHolderName,
		HasLoyaltyCard: cardHasLoyalty != 0,
		LoyaltyPoints:  uint32(cardLoyaltyPts),
	}

	if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		txn.CreatedAt = t
	}
	if completedAt != "" {
		if t, err := time.Parse(time.RFC3339, completedAt); err == nil {
			txn.CompletedAt = &t
		}
	}
	if t, err := time.Parse(time.RFC3339, lastUpdatedStr); err == nil {
		txn.LastUpdated = t
	}

	if len(cachedBlob) > 0 {
		resp := &pb.PaymentCompletionResponse{}
		if err := proto.Unmarshal(cachedBlob, resp); err == nil {
			txn.CachedResponse = resp
		}
	}

	return &txn
}

func insertCompleted(db *sql.DB, txn *Transaction) error {
	completedAt := ""
	if txn.CompletedAt != nil {
		completedAt = txn.CompletedAt.Format(time.RFC3339)
	}

	cardHolderName, cardLastFour := "", ""
	cardHasLoyalty, cardLoyaltyPts := 0, 0
	if txn.Card != nil {
		cardHolderName = txn.Card.HolderName
		if len(txn.Card.CardNumber) >= 4 {
			cardLastFour = txn.Card.CardNumber[len(txn.Card.CardNumber)-4:]
		}
		if txn.Card.HasLoyaltyCard {
			cardHasLoyalty = 1
		}
		cardLoyaltyPts = int(txn.Card.LoyaltyPoints)
	}

	useLoyalty := 0
	if txn.UseLoyaltyPoints {
		useLoyalty = 1
	}

	_, err := db.Exec(`
		INSERT INTO transactions (
			id, state, amount_cents, currency,
			card_holder_name, card_last_four, card_has_loyalty, card_loyalty_points,
			selected_bank_id, selected_aid, installments,
			use_loyalty_points, loyalty_points_used, loyalty_amount_cents, card_amount_cents,
			confirmation_code, receipt_number, auth_code, error_message,
			created_at, completed_at, last_updated
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		txn.ID, int(txn.State), txn.AmountCents, txn.Currency,
		cardHolderName, cardLastFour, cardHasLoyalty, cardLoyaltyPts,
		txn.SelectedBankID, txn.SelectedAID, txn.InstallmentCount,
		useLoyalty, txn.LoyaltyPointsUsed, txn.LoyaltyAmountCents, txn.CardAmountCents,
		txn.ConfirmationCode, txn.ReceiptNumber, txn.AuthCode, txn.ErrorMessage,
		txn.CreatedAt.Format(time.RFC3339), completedAt, txn.LastUpdated.Format(time.RFC3339),
	)
	return err
}
