package pos

import (
	"fmt"
	"math/rand"

	pb "github.com/sirfi/termwire/protocol"
)

// Simulated card data
type CardData struct {
	CardNumber       string
	MaskedNumber     string
	HolderName       string
	ExpiryDate       string
	CVV              string
	AvailableBanks   []*pb.BankApplication
	HasLoyaltyCard   bool
	LoyaltyPoints    uint32
	PointsValueCents uint32
}

// Mock banks database
var mockBanks = []*pb.Bank{
	{Id: 1, Name: "Garanti Bankası"},
	{Id: 2, Name: "İş Bankası"},
	{Id: 3, Name: "Yapı Kredi"},
	{Id: 4, Name: "Akbank"},
	{Id: 5, Name: "Ziraat Bankası"},
}

// Mock bank applications for EMV card
var mockBankApplications = []*pb.BankApplication{
	{
		BankId:          1,
		BankName:        "Garanti Bankası",
		AppLabel:        "GARANTI BONUS",
		Aid:             "A0000000041010",
		Priority:        1,
		SupportsLoyalty: true,
	},
	{
		BankId:          2,
		BankName:        "İş Bankası",
		AppLabel:        "MAXIMUM",
		Aid:             "A0000000042010",
		Priority:        2,
		SupportsLoyalty: true,
	},
	{
		BankId:          3,
		BankName:        "Yapı Kredi",
		AppLabel:        "WORLD",
		Aid:             "A0000000043010",
		Priority:        3,
		SupportsLoyalty: false,
	},
}

// CardReader simulates a card reader device
type CardReader struct {
	currentCard *CardData
}

func NewCardReader() *CardReader {
	return &CardReader{}
}

// SimulateCardInsertion simulates a card being inserted
func (cr *CardReader) SimulateCardInsertion() *CardData {
	// Generate random card data
	cardNumber := fmt.Sprintf("5406%012d", rand.Intn(1000000000000))
	maskedNumber := fmt.Sprintf("540600******%s", cardNumber[len(cardNumber)-4:])

	card := &CardData{
		CardNumber:       cardNumber,
		MaskedNumber:     maskedNumber,
		HolderName:       "JOHN DOE",
		ExpiryDate:       "12/26",
		CVV:              "123",
		AvailableBanks:   mockBankApplications,
		HasLoyaltyCard:   rand.Intn(2) == 1, // 50% chance
		LoyaltyPoints:    uint32(rand.Intn(50000)),
		PointsValueCents: 0,
	}

	// Calculate loyalty points value (1 point = 1 cent in our simulation)
	if card.HasLoyaltyCard {
		card.PointsValueCents = card.LoyaltyPoints
	}

	cr.currentCard = card
	return card
}

// GetCurrentCard returns the currently inserted card
func (cr *CardReader) GetCurrentCard() *CardData {
	return cr.currentCard
}

// RemoveCard simulates card removal
func (cr *CardReader) RemoveCard() {
	cr.currentCard = nil
}

// GetBanks returns list of available banks
func GetBanks() []*pb.Bank {
	return mockBanks
}

// ValidateBank checks if a bank ID is valid
func ValidateBank(bankID uint32) bool {
	for _, bank := range mockBanks {
		if bank.Id == bankID {
			return true
		}
	}
	return false
}

// GetBankApplication returns a specific bank application
func GetBankApplication(aid string) *pb.BankApplication {
	for _, app := range mockBankApplications {
		if app.Aid == aid {
			return app
		}
	}
	return nil
}
