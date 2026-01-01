# Termwire

Termwire is a POS (Point of Sale) terminal communication protocol and implementation written in Go. It provides a robust, frame-based protocol for communication between ECR (Electronic Cash Register) systems and POS terminals.

## Features

- **Frame-Based Protocol**: Binary protocol with CRC32 validation
- **Protobuf Messages**: Efficient message serialization using Protocol Buffers
- **Complete Payment Flow**: Support for card insertion, bank selection, loyalty points, and payment completion
- **Transaction Management**: Support for sales, refunds, and void transactions
- **Reporting**: X reports (without reset) and Z reports (with reset)
- **Gift Cards**: Balance inquiry and charge operations
- **Heartbeat**: Automatic connection monitoring with ping/pong

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
[CRC32]    4 bytes  CRC32 checksum (uint32)
[ETX]      1 byte   0x03
```

### Frame Types

- `PING` (1): Heartbeat request
- `PONG` (2): Heartbeat response
- `DATA` (3): Data frame containing Protobuf messages
- `CLOSE` (4): Connection close request

## Quick Start

### Running the POS Terminal

```bash
# Start the POS terminal server
cd pos/cmd
go run main.go
```

The server will start on `0.0.0.0:8080` by default.

### Running ECR Examples

#### Simple Payment

```bash
cd ecr/examples/simple_payment
go run main.go
```

#### Payment with Loyalty Points

```bash
cd ecr/examples/loyalty_payment
go run main.go
```

#### Refund & Void

```bash
cd ecr/examples/refund_void
go run main.go
```

#### Reports

```bash
cd ecr/examples/reports
go run main.go
```

## Quick Test (Terminal 1 & 2)

### Terminal 1 - Start POS Server

```bash
# Build and run POS terminal
cd /path/to/termwire
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

### Terminal 2 - Run ECR Client

```bash
# In a new terminal
cd /path/to/termwire
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
    // Create and connect client
    config := ecr.DefaultConfig()
    client := ecr.NewClient(config)
    
    if err := client.Connect(); err != nil {
        log.Fatal(err)
    }
    defer client.Disconnect()
    
    // Simple payment
    confirmation, receipt, auth, err := client.SimplePayment(
        "TX-001",      // Transaction ID
        10000,         // Amount in cents (100.00)
        "TRY",         // Currency
        1,             // Bank ID
        "A0000000041010", // AID
        0,             // Installments
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Payment successful! Confirmation: %s", confirmation)
}
```

### Payment Flow with Loyalty

```go
// Start payment
flow, err := client.StartPayment("TX-001", 20000, "TRY")
if err != nil {
    log.Fatal(err)
}

// Get card info
cardMasked, cardHolder := flow.GetCardInfo()
log.Printf("Card: %s, Holder: %s", cardMasked, cardHolder)

// Select bank with loyalty
banks := flow.GetAvailableBanks()
err = flow.SelectBank(banks[0].BankId, banks[0].Aid, 0, true)
if err != nil {
    log.Fatal(err)
}

// Use loyalty points if available
if flow.HasLoyalty() {
    points, value := flow.GetLoyaltyInfo()
    log.Printf("Available: %d points (%.2f TRY)", points, float64(value)/100.0)
    
    // Use half of the points
    if err := flow.ConfirmLoyaltyPoints(points / 2); err != nil {
        log.Fatal(err)
    }
}

// Complete payment
if err := flow.Complete(); err != nil {
    log.Fatal(err)
}

confirmation, receipt, auth := flow.GetResult()
log.Printf("Success! Confirmation: %s", confirmation)
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
    ReadTimeout:          30 * time.Second,
    WriteTimeout:         30 * time.Second,
    MaxTransactionAmount: 10000000, // 100,000.00 in cents
    SupportedCurrencies:  []string{"TRY", "USD", "EUR"},
}
```

### ECR Configuration

```go
config := &ecr.Config{
    ServerHost:        "localhost",
    ServerPort:        8080,
    ConnectTimeout:    10 * time.Second,
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      30 * time.Second,
    HeartbeatInterval: 30 * time.Second,
}
```

## Development

### Prerequisites

- Go 1.21 or later
- Protocol Buffers compiler (`protoc`)
- protoc-gen-go plugin

### Generate Protocol Buffers

```bash
cd protocol
go generate
```

### Build

```bash
# Build POS terminal
cd pos/cmd
go build -o pos-terminal

# Build ECR examples
cd ecr/examples/simple_payment
go build -o simple-payment
```

## Troubleshooting

### Connection Issues

**Problem**: `failed to connect to POS: connection refused`

**Solution**: Ensure POS terminal is running first:
```bash
# Terminal 1
./bin/pos-terminal

# Terminal 2 (in another terminal)
./bin/simple-payment
```

### Port Already in Use

**Problem**: `bind: address already in use`

**Solution**: Change POS port or kill existing process:
```bash
# Kill any running POS terminal
killall pos-terminal

# Or use a different port by modifying config
```

### Frame Validation Errors

**Problem**: `CRC mismatch` or `invalid frame`

**Solution**: Ensure both POS and ECR use compatible protocol versions. Check:
- Protocol version matches (should be 1)
- FLAGS field is properly handled (index 9)
- CRC32 calculation includes all frame fields

## Implementation Notes

- **Card Reader**: Mock implementation for testing
- **Transactions**: Stored in memory (not persistent)
- **Amounts**: Stored in cents (divide by 100 for display)
- **Timestamps**: RFC3339 format
- **Concurrency**: Safe for multiple concurrent connections
- **Error Handling**: Comprehensive error codes and messages
- **Logging**: Debug logging available, controlled by client configuration

## Performance Characteristics

- **Message Latency**: ~1-5ms (localhost testing)
- **Throughput**: ~100+ transactions/sec (single connection)
- **Memory**: ~2MB base + transaction storage
- **Connection Pool**: Unlimited concurrent connections

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
