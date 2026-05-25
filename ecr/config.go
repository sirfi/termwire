package ecr

import (
	"log/slog"
	"time"
)

// Config holds the ECR client configuration
type Config struct {
	// POS connection settings
	POSHost string
	POSPort int

	// Connection timeouts
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration

	// Retry settings
	MaxRetries int
	RetryDelay time.Duration

	// Client information
	ClientID      string
	ClientVersion string

	// Transaction settings
	DefaultCurrency string
	EnableLoyalty   bool

	// TLS configuration (optional — leave empty to use plain TCP)
	TLSEnabled    bool
	TLSCAFile     string // CA cert to verify the server certificate
	TLSCertFile   string // client certificate for mTLS (optional)
	TLSKeyFile    string // client private key for mTLS (optional)
	TLSServerName string // SNI / server name override (defaults to POSHost)

	// Logging
	Debug    bool
	LogLevel slog.Level
}

// DefaultConfig returns a default ECR configuration
func DefaultConfig() *Config {
	return &Config{
		POSHost:         "localhost",
		POSPort:         8080,
		ConnectTimeout:  10 * time.Second,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		MaxRetries:      3,
		RetryDelay:      2 * time.Second,
		ClientID:        "ECR-001",
		ClientVersion:   "1.0.0",
		DefaultCurrency: "TRY",
		EnableLoyalty:   true,
		Debug:           false,
		TLSEnabled:      false,
		LogLevel:        slog.LevelInfo,
	}
}

// WithPOSAddress sets the POS server address
func (c *Config) WithPOSAddress(host string, port int) *Config {
	c.POSHost = host
	c.POSPort = port
	return c
}

// WithTimeouts sets connection timeouts
func (c *Config) WithTimeouts(connect, read, write time.Duration) *Config {
	c.ConnectTimeout = connect
	c.ReadTimeout = read
	c.WriteTimeout = write
	return c
}

// WithRetry sets retry configuration
func (c *Config) WithRetry(maxRetries int, delay time.Duration) *Config {
	c.MaxRetries = maxRetries
	c.RetryDelay = delay
	return c
}

// WithClientInfo sets client identification
func (c *Config) WithClientInfo(id, version string) *Config {
	c.ClientID = id
	c.ClientVersion = version
	return c
}

// WithCurrency sets the default currency
func (c *Config) WithCurrency(currency string) *Config {
	c.DefaultCurrency = currency
	return c
}

// WithLoyalty enables or disables loyalty support
func (c *Config) WithLoyalty(enabled bool) *Config {
	c.EnableLoyalty = enabled
	return c
}

// WithDebug enables or disables debug logging
func (c *Config) WithDebug(enabled bool) *Config {
	c.Debug = enabled
	return c
}
