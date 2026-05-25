# Termwire

Termwire is a **demonstrative** POS (Point of Sale) terminal communication protocol and implementation written in Go. It provides a robust, frame-based protocol for communication between ECR (Electronic Cash Register) systems and POS terminals.

> **Note**: This is an example protocol implementation created for educational and demonstration purposes. It showcases best practices in protocol design, Go programming, and system architecture.

## Features

- **Frame-Based Protocol**: Binary protocol with CRC32 validation
- **Protobuf Messages**: Efficient message serialization using Protocol Buffers
- **Complete Payment Flow**: Support for card insertion, bank selection, loyalty points, and payment completion
- **Transaction Management**: Support for sales, refunds, and void transactions; completed transactions persisted in SQLite
- **Reporting**: X reports (current batch totals) and Z reports (batch close — history preserved, new batch started)
- **Gift Cards**: Balance inquiry and charge operations
- **Heartbeat**: Automatic connection monitoring with ping/pong
- **TLS/mTLS**: Optional TLS 1.3 encryption; mutual TLS when `TLSCAFile` is set on both sides
- **Structured Logging**: JSON-format logs via `log/slog`; level controlled per-component

## Architecture

### Components

1. **Protocol** (`protocol/`): Core protocol implementation
   - Frame serialization/deserialization
   - CRC32 validation
   - Protobuf message definitions

2. **POS** (`pos/`): POS terminal server implementation
   - TCP server for handling ECR connections
   - Transaction management
   - Card reader simulation
   - Report generation

3. **ECR** (`ecr/`): ECR client implementation
   - TCP client for connecting to POS terminals
   - High-level payment API
   - Multi-step payment flow management
   - Example applications

## Protocol Structure

Each frame has the following structure:

```
[STX]      1 byte   0x02
[MAGIC]    2 bytes  'TW' (0x54 0x57)
[VER]      1 byte   Protocol version
[LEN]      2 bytes  Payload length (uint16)
[TYPE]     1 byte   Frame type
[SEQ]      2 bytes  Sequence number (uint16)
[FLAGS]    1 byte   Reserved
[PAYLOAD]  N bytes  Message payload (Protobuf)
[CRC32]    4 bytes  CRC32 checksum (uint32) — covers VER, TYPE, SEQ, PAYLOAD
[ETX]      1 byte   0x03
```

### Frame Types

- `PING` (1): Heartbeat request
- `PONG` (2): Heartbeat response
- `DATA` (3): Data frame containing Protobuf messages
- `CLOSE` (4): Connection close request

## Transaction State Machine

Each payment on the POS side progresses through the following states:

```
CardInsertionRequest
        │
        ▼
 CARD_INSERTION
        │ card read OK
        ▼
 BANK_SELECTION ──────────────────────────────────────────────────────┐
        │ PaymentProcessingRequest                                     │
        │ (use_loyalty_points=false OR card has no loyalty)            │
        ▼                                                              │
 PAYMENT_PROCESSING ◄─────────────────────────────────────────────────┤
        │ PaymentProcessingRequest (use_loyalty_points=true,           │
        │                           card has loyalty)                  │
        │                                                              │
        │                                                   LOYALTY_INQUIRY
        │                                                              │ LoyaltyPointsConfirmation
        │                                                              ▼
        │                                                  LOYALTY_CONFIRMATION
        │                                                              │
        │                                                              ▼
        │                                                   PAYMENT_PROCESSING
        │
        ├── success ──► PAYMENT_COMPLETED ──► VOIDED
        │                                └──► REFUNDED
        │
        └── declined ──► FAILED
```

- Active transactions (states before `PAYMENT_COMPLETED`) are held **in-memory** only.
- Terminal states (`PAYMENT_COMPLETED`, `FAILED`, `VOIDED`, `REFUNDED`) are **persisted to SQLite**.
- The `message_id` on requests acts as an idempotency key — duplicate requests return the cached response.

## Quick Start

### One-liner (recommended)

The `run-example.sh` script builds everything, starts the POS server in the background, waits for it to become ready, runs the chosen example, and stops the server automatically on exit.

```bash
# Simple payment (requires TLS certs — run gen-certs.sh first)
bash scripts/gen-certs.sh
bash scripts/run-example.sh simple_payment

# Other examples (plain TCP, no certs needed)
bash scripts/run-example.sh loyalty_payment
bash scripts/run-example.sh refund_void
bash scripts/run-example.sh reports
```

### Running the POS Terminal

```bash
go build -o bin/pos-terminal ./pos/cmd/
./bin/pos-terminal
```

The server will start on `0.0.0.0:8080` by default.

### Running ECR Examples

```bash
# Simple payment
go build -o bin/simple-payment ./ecr/examples/simple_payment/
./bin/simple-payment

# Payment with loyalty points
go run ./ecr/examples/loyalty_payment/

# Refund & void
go run ./ecr/examples/refund_void/

# Reports
go run ./ecr/examples/reports/
```

## Quick Test (Terminal 1 & 2)

### Terminal 1 — Start POS Server

```bash
go build -o bin/pos-terminal ./pos/cmd/
./bin/pos-terminal
```

Expected output:
```
=== Termwire POS Terminal ===
Configuration:
   Terminal ID: POS-001
   Serial Number: SN-123456789
   Version: 1.0.0
   Address: 0.0.0.0:8080
POS terminal is ready to accept connections...
```

### Terminal 2 — Run ECR Client

```bash
go build -o bin/simple-payment ./ecr/examples/simple_payment/
./bin/simple-payment
```

Expected output:
```
=== Simple Payment Example ===
Connected successfully!
Terminal Version: 1.0.0
...
==================================================
                    RECEIPT
==================================================
Transaction ID:    TXN-SIMPLE-001
Receipt Number:    RCP-1001
Confirmation Code: CONF-1767293626
...
==================================================
Simple payment example completed successfully!
```

## Usage

### ECR Client Example

```go
package main

import (
    "log"
    "github.com/sirfi/termwire/ecr"
)

func main() {
    config := ecr.DefaultConfig() // connects to localhost:8080
    api := ecr.NewAPI(config)

    if err := api.Connect(); err != nil {
        log.Fatal(err)
    }
    defer api.Disconnect()

    flow := ecr.NewPaymentFlow(api)
    result, err := flow.ExecuteSimplePayment(
        "TX-001", // Transaction ID (idempotency key — preserve on retries)
        10000,    // Amount in cents (100.00 TRY)
        "TRY",    // Currency (ISO 4217)
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Payment successful! Confirmation: %s", result.ConfirmationCode)
    log.Printf("Receipt: %s, Auth: %s", result.ReceiptNumber, result.AuthCode)
}
```

### Payment Flow with Loyalty (high-level)

```go
// ExecuteLoyaltyPayment handles the full loyalty inquiry + confirmation flow automatically.
// Pass 0 as loyaltyPoints to use all available points; or pass a specific amount to cap usage.
result, err := flow.ExecuteLoyaltyPayment("TX-001", 20000, "TRY", 0)
if err != nil {
    log.Fatal(err)
}

log.Printf("Card charged: %.2f TRY", float64(result.CardAmountCents)/100.0)
log.Printf("Loyalty discount: %.2f TRY", float64(result.LoyaltyAmountCents)/100.0)
log.Printf("Remaining points: %d", result.RemainingPoints)
```

### Payment Flow with Loyalty (manual, fine-grained)

```go
// Step 1: Insert card and get available bank applications
cardResp, err := api.InsertCard("TX-001", 20000, "TRY")
if err != nil {
    log.Fatal(err)
}
log.Printf("Card: %s, Holder: %s", cardResp.CardNumberMasked, cardResp.CardHolderName)

bank := cardResp.AvailableBanks[0] // pick preferred bank

// Step 2: Send bank selection; receive loyalty inquiry if card supports it
paymentResp, loyaltyResp, err := api.ProcessPayment(
    "TX-001", bank.BankId, bank.Aid, 0 /*installments*/, bank.SupportsLoyalty)
if err != nil {
    log.Fatal(err)
}

// Step 3: Handle loyalty — only present when loyaltyResp != nil
if loyaltyResp != nil {
    log.Printf("Available: %d points (%.2f TRY)",
        loyaltyResp.AvailablePoints, float64(loyaltyResp.PointsValueCents)/100.0)

    pointsToUse := loyaltyResp.AvailablePoints / 2
    paymentResp, err = api.ConfirmLoyaltyPoints("TX-001", pointsToUse, pointsToUse)
    if err != nil {
        log.Fatal(err)
    }
}

log.Printf("Success! Confirmation: %s", paymentResp.ConfirmationCode)
```

## Message Types

### Information Requests
- `GetVersionRequest/Response`: Get protocol version
- `GetTerminalInfoRequest/Response`: Get terminal information
- `GetBanksRequest/Response`: Get available banks

### Payment Flow
- `CardInsertionRequest/Response`: Card insertion and reading
- `PaymentProcessingRequest`: Bank and loyalty selection
- `LoyaltyCardInquiryResponse`: Loyalty points information
- `LoyaltyPointsConfirmation`: Confirm loyalty points usage
- `PaymentCompletionRequest/Response`: Complete payment

### Transaction Operations
- `RefundTransactionRequest/Response`: Process refunds
- `VoidTransactionRequest/Response`: Cancel transactions

### Reports
- `XReportRequest/Response`: Current day totals (no reset)
- `ZReportRequest/Response`: End of day totals (with reset)
- `DetailedReportRequest/Response`: Transaction history

### Gift Cards
- `GiftCardInquiryRequest/Response`: Check balance
- `GiftCardChargeRequest/Response`: Load/charge gift card

## Configuration

### POS Configuration

```go
config := &pos.Config{
    Host:                 "0.0.0.0",
    Port:                 8080,
    TerminalID:           "POS-001",
    SerialNumber:         "SN-123456789",
    Version:              "1.0.0",
    SoftwareVendor:       "Sirfi Technologies",
    ReadTimeout:          30 * time.Second,
    WriteTimeout:         30 * time.Second,
    IdleTimeout:          5 * time.Minute,
    MaxTransactionAmount: 10000000, // 100,000.00 in cents
    SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
    // TLS (optional)
    TLSEnabled:           false,
    TLSCertFile:          "certs/server.crt",
    TLSKeyFile:           "certs/server.key",
    TLSCAFile:            "certs/ca.crt", // enables mTLS client verification
    // Database
    DBFile:               "pos-terminal.db", // use ":memory:" for tests
    LogLevel:             slog.LevelInfo,
}
```

### ECR Configuration

```go
config := &ecr.Config{
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
    // TLS (optional)
    TLSEnabled:      false,
    TLSCAFile:       "certs/ca.crt",
    TLSCertFile:     "certs/client.crt", // for mTLS
    TLSKeyFile:      "certs/client.key", // for mTLS
    TLSServerName:   "localhost",
    LogLevel:        slog.LevelInfo,
}
```

### POS Environment Variables

The `pos/cmd` binary reads the following environment variables at startup. All are optional; unset variables fall back to `DefaultConfig()` values.

| Variable | Default | Description |
|----------|---------|-------------|
| `POS_HOST` | `0.0.0.0` | TCP bind address |
| `POS_PORT` | `8080` | TCP port |
| `POS_TERMINAL_ID` | `POS-001` | Terminal identifier string |
| `POS_TLS_ENABLED` | `false` | Set to `true` to enable TLS |
| `POS_TLS_CERT` | `certs/server.crt` | Server certificate (PEM) — used when TLS enabled |
| `POS_TLS_KEY` | `certs/server.key` | Server private key (PEM) — used when TLS enabled |
| `POS_TLS_CA` | `certs/ca.crt` | CA certificate — enables mTLS when set |
| `POS_DB_FILE` | `pos-terminal.db` | SQLite database path; use `:memory:` for ephemeral runs |

```bash
# Example: run with mTLS on a custom port, separate DB
POS_TLS_ENABLED=true POS_PORT=9090 POS_DB_FILE=/data/txn.db ./bin/pos-terminal
```

## TLS / mTLS

TLS is disabled by default. To enable, set `TLSEnabled: true` and supply certificate paths. When `TLSCAFile` is configured on **both** sides, mutual TLS (mTLS) is enforced. Minimum protocol version is TLS 1.3.

Generate self-signed certificates for local testing:

```bash
bash scripts/gen-certs.sh
```

This creates a `certs/` directory containing:

| File | Purpose |
|------|---------|
| `ca.crt` | CA certificate — used by both sides |
| `server.crt` / `server.key` | POS server certificate |
| `client.crt` / `client.key` | ECR client certificate (mTLS) |

## Development

### Prerequisites

- Go 1.21 or later
- `protoc` + `protoc-gen-go` (for regenerating Protobuf)
- `buf` (for proto linting — `buf.yaml` is present)
- `openssl` (for `gen-certs.sh`)

### Generate Protocol Buffers

```bash
# From project root
go generate ./protocol/
```

### Build

```bash
# POS terminal
go build -o bin/pos-terminal ./pos/cmd/

# ECR simple payment example
go build -o bin/simple-payment ./ecr/examples/simple_payment/
```

## Troubleshooting

### Connection Issues

**Problem**: `failed to connect to POS: connection refused`

**Solution**: Start the POS terminal first, then the ECR client.

```bash
# Terminal 1
./bin/pos-terminal

# Terminal 2
./bin/simple-payment
```

### Port Already in Use

**Problem**: `bind: address already in use`

**Solution**: Kill the existing process or change the port in the config.

```bash
killall pos-terminal
```

### Frame Validation Errors

**Problem**: `CRC mismatch` or `invalid frame`

**Solution**: Ensure both POS and ECR use the same protocol version. CRC32 covers the version, type, seq, and payload fields of each frame.

## Testing

### Running Unit Tests

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -cover ./...

# Run specific packages
go test -v ./protocol/
go test -v ./pos/
go test -v ./ecr/
```

### Test Coverage

- **Protocol** (`protocol_test.go`): frame validation, CRC32, length calculation
- **POS Card** (`pos/card_test.go`): card reader simulation and bank validation
- **POS Transaction** (`pos/transaction_test.go`): transaction state management
- **ECR Client** (`ecr/client_test.go`): client initialization and sequence handling
- **ECR Payment** (`ecr/payment_test.go`): payment structures

**Total**: 39 unit tests, all passing

### Integration Testing

Run the POS terminal and an ECR example in separate terminals (see [Quick Test](#quick-test-terminal-1--2) above).

## Implementation Notes

- **Card Reader**: Mock implementation — no real hardware required
- **Transactions**: Completed transactions persisted in SQLite (`pos-terminal.db`); active transactions in-memory
- **Z-Report**: Closes the current batch (`z_report_id`); transaction history preserved across batches
- **Amounts**: Always in cents — divide by 100 for display
- **Timestamps**: RFC3339 format
- **Idempotency**: `message_id` is preserved on retries; duplicate requests are deduplicated
- **Concurrency**: Safe for multiple concurrent ECR connections
- **Logging**: `log/slog` with JSON handler; level set via `Config.LogLevel`

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
