# Termwire — Copilot Instructions

Demonstrative POS terminal communication protocol in Go. Educational/demo project — not production code.

## Module

```
github.com/sirfi/termwire
```

## Package Responsibilities

| Package | Role |
|---------|------|
| `protocol/` | Binary frame codec (CRC32) + Protobuf types. `termwire.pb.go` is generated — never edit it directly. |
| `pos/` | TCP server simulating a POS terminal (transactions, card reader, reports). |
| `ecr/` | TCP client library for ECR systems. High-level payment API + example programs. |
| `termwire/v1/` | Canonical `.proto` source. Run `go generate ./protocol/` to regenerate. |

## Non-Negotiable Conventions

- **Amounts** are always integers in **cents**. Never use floats for money. Divide by 100 only for display.
- **Timestamps** use `time.RFC3339` format everywhere.
- **`message_id`** must be preserved on retries — this is the idempotency key the server uses to return cached responses.
- **TLS** minimum version is 1.3. mTLS is enabled when `TLSCAFile` is set on both sides.
- **Logging** uses `log/slog` with a JSON handler. Level is controlled via `Config.LogLevel`. Do not use `fmt.Println` or `log.Printf` for runtime logging.
- **Transactions** are in-memory only. Do not add persistence unless explicitly asked.

## Wire Protocol Frame

```
[STX 1B][MAGIC 2B='TW'][VER 1B][LEN 2B][TYPE 1B][SEQ 2B][FLAGS 1B][PAYLOAD NB][CRC32 4B][ETX 1B]
```

Frame types: `PING`(1) `PONG`(2) `DATA`(3) `CLOSE`(4).  
CRC32 input: version + type + seq + payload.

## Testing

37 unit tests, no external dependencies. All tests are self-contained.  
Run: `go test -v ./...`
