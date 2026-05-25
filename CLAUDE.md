# Termwire

Demonstrative POS terminal communication protocol in Go. Educational/demo project — not production.

## Architecture

- **`protocol/`** — Binary frame codec (CRC32) + Protobuf message definitions. `protocol.go` + generated `termwire.pb.go`.
- **`pos/`** — TCP server simulating a POS terminal. Handles ECR connections, transactions, card reader, reports.
- **`ecr/`** — TCP client library for ECR (Electronic Cash Register). High-level payment API + examples.
- **`termwire/v1/`** — Canonical `.proto` source. The generated Go file lives in `protocol/`.

## Module

```
github.com/sirfi/termwire
```

## Commands

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -cover ./...

# Build POS server
go build -o bin/pos-terminal ./pos/cmd/

# Build ECR example
go build -o bin/simple-payment ./ecr/examples/simple_payment/

# Run POS server (Terminal 1)
./bin/pos-terminal        # listens on 0.0.0.0:8080

# Run ECR example (Terminal 2)
./bin/simple-payment

# Regenerate protobuf (from protocol/ dir)
go generate ./protocol/
```

## Wire Protocol Frame Layout

```
[STX 1B][MAGIC 2B='TW'][VER 1B][LEN 2B][TYPE 1B][SEQ 2B][FLAGS 1B][PAYLOAD NB][CRC32 4B][ETX 1B]
```

Frame types: `PING`(1) `PONG`(2) `DATA`(3) `CLOSE`(4). CRC32 covers: version, type, seq, payload.

## Key Conventions

- Amounts are always in **cents** (integer). Divide by 100 for display.
- Timestamps use **RFC3339** format.
- `message_id` is used for idempotency — retries preserve the original ID.
- TLS 1.3 minimum; mTLS supported when `TLSCAFile` is set on both sides.
- Transactions are in-memory only (no persistence).
- Logging uses `log/slog` with JSON handler; level controlled via `Config.LogLevel`.

## Proto → Go

Proto source: `termwire/v1/termwire.proto`  
Generated output: `protocol/termwire.pb.go` (option `go_package = "github.com/sirfi/termwire/protocol"`)  
Use `buf` for linting (`buf.yaml` present). Regenerate with `go generate ./protocol/`.

## Testing

37 unit tests across 5 files. No database or external dependencies — all tests are self-contained.
Integration testing: run POS server + ECR example in separate terminals.
