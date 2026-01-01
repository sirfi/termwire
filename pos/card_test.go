package pos

import (
	"testing"
)

func TestSimulateCardInsertion(t *testing.T) {
	cr := NewCardReader()
	card := cr.SimulateCardInsertion()

	if card == nil {
		t.Fatal("SimulateCardInsertion should return card")
	}

	if card.CardNumber == "" {
		t.Error("CardNumber should not be empty")
	}

	if len(card.AvailableBanks) == 0 {
		t.Error("AvailableBanks should not be empty")
	}
}

func TestGetCurrentCard(t *testing.T) {
	cr := NewCardReader()

	if cr.GetCurrentCard() != nil {
		t.Error("Card should be nil before insertion")
	}

	cr.SimulateCardInsertion()
	card := cr.GetCurrentCard()
	if card == nil {
		t.Fatal("Card should be present after insertion")
	}
}

func TestRemoveCard(t *testing.T) {
	cr := NewCardReader()
	cr.SimulateCardInsertion()

	if cr.GetCurrentCard() == nil {
		t.Fatal("Card should exist before removal")
	}

	cr.RemoveCard()

	if cr.GetCurrentCard() != nil {
		t.Error("Card should be nil after removal")
	}
}

func TestValidateBank(t *testing.T) {
	tests := []struct {
		bankID    uint32
		shouldFit bool
	}{
		{1, true},
		{5, true},
		{99, false},
	}

	for _, tt := range tests {
		result := ValidateBank(tt.bankID)
		if result != tt.shouldFit {
			t.Errorf("ValidateBank(%d): got %v, want %v", tt.bankID, result, tt.shouldFit)
		}
	}
}

func TestGetBankApplication(t *testing.T) {
	cr := NewCardReader()
	cr.SimulateCardInsertion()

	app := GetBankApplication("A0000000041010")
	if app == nil {
		t.Fatal("Should find bank application")
	}

	if app.BankId != 1 {
		t.Errorf("BankId: got %d, want 1", app.BankId)
	}

	invalidApp := GetBankApplication("INVALID")
	if invalidApp != nil {
		t.Error("Should not find invalid bank")
	}
}
