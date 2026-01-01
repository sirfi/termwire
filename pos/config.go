package pos

import "time"

type Config struct {
	// Network configuration
	Host string
	Port int

	// Terminal information
	TerminalID     string
	SerialNumber   string
	Version        string
	SoftwareVendor string

	// Connection settings
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Transaction settings
	MaxTransactionAmount int // in cents
	SupportedCurrencies  []string
}

func DefaultConfig() *Config {
	return &Config{
		Host:                 "0.0.0.0",
		Port:                 8080,
		TerminalID:           "POS-001",
		SerialNumber:         "SN-123456789",
		Version:              "1.0.0",
		SoftwareVendor:       "Sirfi Technologies",
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		IdleTimeout:          5 * time.Minute,
		MaxTransactionAmount: 10000000, // 100,000.00
		SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
	}
}
