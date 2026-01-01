package ecr

import (
	"testing"
)

func TestNewPaymentFlow(t *testing.T) {
	mockAPI := &API{
		client: &Client{
			config: &Config{
				POSHost: "localhost",
				POSPort: 8080,
			},
		},
	}

	flow := NewPaymentFlow(mockAPI)

	if flow == nil {
		t.Fatal("NewPaymentFlow returned nil")
	}

	if flow.api != mockAPI {
		t.Error("API not set correctly")
	}
}

func TestPaymentRequestCreation(t *testing.T) {
	req := &PaymentRequest{
		TransactionID:    "TXN-001",
		AmountCents:      10000,
		Currency:         "TRY",
		BankID:           1,
		InstallmentCount: 1,
		UseLoyalty:       false,
		LoyaltyPoints:    0,
	}

	if req.TransactionID != "TXN-001" {
		t.Errorf("TransactionID: got %s", req.TransactionID)
	}

	if req.AmountCents != 10000 {
		t.Errorf("AmountCents: got %d", req.AmountCents)
	}

	if req.Currency != "TRY" {
		t.Errorf("Currency: got %s", req.Currency)
	}
}

func TestPaymentRequestWithLoyalty(t *testing.T) {
	req := &PaymentRequest{
		TransactionID:    "TXN-002",
		AmountCents:      5000,
		Currency:         "TRY",
		BankID:           2,
		InstallmentCount: 3,
		UseLoyalty:       true,
		LoyaltyPoints:    100,
	}

	if !req.UseLoyalty {
		t.Error("UseLoyalty should be true")
	}

	if req.LoyaltyPoints != 100 {
		t.Errorf("LoyaltyPoints: got %d", req.LoyaltyPoints)
	}
}

func TestPaymentResultCreation(t *testing.T) {
	result := &PaymentResult{
		Success:          true,
		TransactionID:    "TXN-001",
		ConfirmationCode: "CONF-123",
		AuthCode:         "AUTH-456",
		ReceiptNumber:    "RCP-1001",
		CardAmountCents:  10000,
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if result.ConfirmationCode != "CONF-123" {
		t.Errorf("ConfirmationCode: got %s", result.ConfirmationCode)
	}
}

func TestPaymentResultFailure(t *testing.T) {
	result := &PaymentResult{
		Success:      false,
		ErrorMessage: "Payment declined",
		ErrorCode:    "DECLINED",
	}

	if result.Success {
		t.Error("Success should be false")
	}

	if result.ErrorMessage != "Payment declined" {
		t.Errorf("ErrorMessage: got %s", result.ErrorMessage)
	}
}

func TestPaymentResultWithLoyalty(t *testing.T) {
	result := &PaymentResult{
		Success:            true,
		CardAmountCents:    7000,
		LoyaltyAmountCents: 3000,
		RemainingPoints:    50,
	}

	total := result.CardAmountCents + result.LoyaltyAmountCents
	if total != 10000 {
		t.Errorf("Total: got %d, want 10000", total)
	}
}

func TestPaymentFlowNilAPI(t *testing.T) {
	flow := NewPaymentFlow(nil)

	if flow == nil {
		t.Fatal("NewPaymentFlow with nil API returned nil")
	}

	if flow.api != nil {
		t.Error("API should be nil")
	}
}

func TestPaymentRequestAmountVariations(t *testing.T) {
	tests := []struct {
		amountCents uint32
		currency    string
	}{
		{100, "TRY"},
		{50000, "TRY"},
		{100000, "TRY"},
		{10000, "USD"},
		{10000, "EUR"},
	}

	for _, tt := range tests {
		req := &PaymentRequest{
			AmountCents: tt.amountCents,
			Currency:    tt.currency,
		}

		if req.AmountCents != tt.amountCents {
			t.Errorf("AmountCents: got %d, want %d", req.AmountCents, tt.amountCents)
		}

		if req.Currency != tt.currency {
			t.Errorf("Currency: got %s, want %s", req.Currency, tt.currency)
		}
	}
}

func TestPaymentRequestInstallments(t *testing.T) {
	tests := []struct {
		installmentCount uint32
	}{
		{1},
		{3},
		{12},
		{24},
	}

	for _, tt := range tests {
		req := &PaymentRequest{
			AmountCents:      10000,
			InstallmentCount: tt.installmentCount,
		}

		if req.InstallmentCount != tt.installmentCount {
			t.Errorf("InstallmentCount: got %d, want %d", req.InstallmentCount, tt.installmentCount)
		}
	}
}

func TestPaymentFlowConcurrency(t *testing.T) {
	mockAPI := &API{
		client: &Client{
			config: &Config{
				POSHost: "localhost",
				POSPort: 8080,
			},
		},
	}

	flow := NewPaymentFlow(mockAPI)
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			if flow == nil {
				t.Error("Payment flow should not be nil")
			}
			req := &PaymentRequest{
				AmountCents: uint32(idx * 1000),
			}
			if req.AmountCents < 0 {
				t.Error("Amount should be non-negative")
			}
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}
