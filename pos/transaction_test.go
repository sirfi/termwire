package pos

import (
	"testing"
	"time"
)

func TestTransactionStateString(t *testing.T) {
	tests := []struct {
		state    TransactionState
		expected string
	}{
		{StateCardInsertion, "CARD_INSERTION"},
		{StateBankSelection, "BANK_SELECTION"},
		{StatePaymentCompleted, "PAYMENT_COMPLETED"},
		{StateFailed, "FAILED"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("String(): got %s, want %s", tt.state.String(), tt.expected)
		}
	}
}

func TestNewTransactionManager(t *testing.T) {
	config := &Config{
		TerminalID:           "TEST-001",
		MaxTransactionAmount: 999999,
		SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
	}
	tm := NewTransactionManager(config)

	if tm == nil {
		t.Fatal("NewTransactionManager returned nil")
	}

	if tm.config != config {
		t.Error("Config not set correctly")
	}
}

func TestCreateTransaction(t *testing.T) {
	tm := NewTransactionManager(&Config{
		TerminalID:           "TEST-001",
		MaxTransactionAmount: 999999,
		SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
	})

	tx, err := tm.CreateTransaction("TXN-001", 10000, "TRY")
	if err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	if tx == nil {
		t.Fatal("CreateTransaction returned nil")
	}

	if tx.ID == "" {
		t.Error("Transaction ID should not be empty")
	}

	if tx.AmountCents != 10000 {
		t.Errorf("Amount: got %d, want 10000", tx.AmountCents)
	}

	if tx.Currency != "TRY" {
		t.Errorf("Currency: got %s, want TRY", tx.Currency)
	}

	if tx.State != StateCardInsertion {
		t.Errorf("Initial state: got %v, want StateCardInsertion", tx.State)
	}
}

func TestGetTransaction(t *testing.T) {
	tm := NewTransactionManager(&Config{
		TerminalID:           "TEST-001",
		MaxTransactionAmount: 999999,
		SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
	})
	tx, err := tm.CreateTransaction("TXN-001", 5000, "TRY")
	if err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	retrieved, err := tm.GetTransaction(tx.ID)
	if err != nil {
		t.Fatalf("GetTransaction failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetTransaction returned nil")
	}

	if retrieved.ID != tx.ID {
		t.Errorf("Transaction ID mismatch: got %s, want %s", retrieved.ID, tx.ID)
	}
}

func TestUpdateTransactionState(t *testing.T) {
	tm := NewTransactionManager(&Config{
		TerminalID:           "TEST-001",
		MaxTransactionAmount: 999999,
		SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
	})
	tx, err := tm.CreateTransaction("TXN-001", 5000, "TRY")
	if err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	err = tm.UpdateTransactionState(tx.ID, StateBankSelection)
	if err != nil {
		t.Fatalf("UpdateTransactionState failed: %v", err)
	}

	tx, err = tm.GetTransaction(tx.ID)
	if err != nil {
		t.Fatalf("GetTransaction failed: %v", err)
	}

	if tx.State != StateBankSelection {
		t.Errorf("State: got %v, want StateBankSelection", tx.State)
	}
}

func TestCompleteTransaction(t *testing.T) {
	tm := NewTransactionManager(&Config{
		TerminalID:           "TEST-001",
		MaxTransactionAmount: 999999,
		SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
	})
	tx, err := tm.CreateTransaction("TXN-COMPLETE", 5000, "TRY")
	if err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Complete the transaction
	err = tm.CompleteTransaction(tx.ID, "CONF-123", "AUTH-456")
	if err != nil {
		t.Errorf("CompleteTransaction failed: %v", err)
	}
}

func TestFailTransaction(t *testing.T) {
	tm := NewTransactionManager(&Config{
		TerminalID:           "TEST-001",
		MaxTransactionAmount: 999999,
		SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
	})
	tx, err := tm.CreateTransaction("TXN-FAIL", 5000, "TRY")
	if err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Fail the transaction
	err = tm.FailTransaction(tx.ID, "Payment declined")
	if err != nil {
		t.Errorf("FailTransaction failed: %v", err)
	}
}

func TestTransactionConcurrency(t *testing.T) {
	tm := NewTransactionManager(&Config{
		TerminalID:           "TEST-001",
		MaxTransactionAmount: 999999,
		SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
	})
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			tx, err := tm.CreateTransaction("TXN-"+string(rune('0'+idx)), 1000, "TRY")
			if err != nil {
				t.Errorf("CreateTransaction failed: %v", err)
			}
			if tx == nil {
				t.Error("CreateTransaction returned nil")
			}
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	if len(tm.activeTransactions) != 5 {
		t.Errorf("Active transactions: got %d, want 5", len(tm.activeTransactions))
	}
}

func TestTransactionTimestamps(t *testing.T) {
	tm := NewTransactionManager(&Config{
		TerminalID:           "TEST-001",
		MaxTransactionAmount: 999999,
		SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
	})
	before := time.Now()
	tx, err := tm.CreateTransaction("TXN-001", 1000, "TRY")
	if err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}
	after := time.Now()

	if tx.CreatedAt.Before(before) || tx.CreatedAt.After(after) {
		t.Error("CreatedAt should be between before and after")
	}
}
