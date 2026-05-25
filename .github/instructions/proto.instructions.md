---
applyTo: "**/*.proto"
---

# Proto Conventions — Termwire

## Package & Options

```protobuf
syntax = "proto3";
package termwire.v1;

option go_package = "github.com/sirfi/termwire/protocol";
```

Always keep all four language options (`go_package`, `java_package`, `java_outer_classname`, `csharp_namespace`) in sync when adding new files.

## Message Envelope

All messages go through the top-level `Message` wrapper using a `oneof body` field. New request/response pairs must be added to this oneof with consecutive field numbers.

```protobuf
message Message {
  string message_id = 1;   // idempotency key — preserve on retries
  string timestamp  = 2;   // RFC3339 string

  oneof body {
    MyNewRequest  my_new_request  = <next_number>;
    MyNewResponse my_new_response = <next_number + 1>;
  }
}
```

## Field Conventions

| Data | Proto type | Notes |
|------|-----------|-------|
| Money | `uint32 *_cents` | always cents, never float |
| Timestamp | `string` | RFC3339 format |
| Response status | `string code` | first field in every response |
| Human message | `string message` | last field in every response |

## Naming

- Request messages: `VerbNounRequest` (e.g. `CardInsertionRequest`)
- Response messages: `VerbNounResponse` (e.g. `CardInsertionResponse`)
- Enums: `UPPER_SNAKE_CASE`, prefixed with enum name (e.g. `TRANSACTION_TYPE_SALE`)
- Unspecified zero value required for every enum: `ENUM_NAME_UNSPECIFIED = 0`

## Regeneration

```bash
go generate ./protocol/
```

This runs `protoc` via the directive in `protocol/protocol.go`. Requires `protoc` and `protoc-gen-go` installed.
