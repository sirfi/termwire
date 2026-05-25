---
applyTo: "**/*.go"
---

# Go Conventions — Termwire

## Logging

Use `log/slog` exclusively. Never use `fmt.Println`, `log.Print*`, or `log.Fatal*` in library packages.

```go
// correct
logger.Info("client connected", slog.String("client_id", id), slog.String("addr", addr))
logger.Error("frame error", slog.Any("error", err))

// wrong
log.Printf("client connected: %s", id)
fmt.Println("connected")
```

## Error Handling

Wrap errors with context. Return, don't log-and-return.

```go
// correct
return fmt.Errorf("failed to read CA file: %w", err)

// wrong
log.Printf("error: %v", err)
return err
```

## Concurrency

- Use `sync.RWMutex` for read-heavy shared state (connection maps, config reads).
- Use `sync.Mutex` for write-heavy or simple mutual exclusion (sequence counters).
- Always defer unlock immediately after locking.

## Money / Amounts

All amounts are `uint32` in cents. Never use `float64` for monetary values.

```go
// correct — display only
fmt.Sprintf("%.2f %s", float64(amountCents)/100.0, currency)

// wrong — storage or calculation
price := 19.99
```

## Timestamps

```go
time.Now().Format(time.RFC3339)   // correct
time.Now().String()               // wrong
```

## Message IDs

Generate with a stable, unique prefix + identifier. Preserve on retries.

```go
msgID := fmt.Sprintf("TXN-%s-%d", transactionID, time.Now().UnixNano())
```

## Protobuf

- Never edit `protocol/termwire.pb.go` directly.
- To add/modify messages, edit `termwire/v1/termwire.proto` and run `go generate ./protocol/`.
- Use the `pb` import alias: `pb "github.com/sirfi/termwire/protocol"`.
