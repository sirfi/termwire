# Termwire Protocol Specification

## Overview

Termwire is a lightweight, binary protocol designed for reliable terminal communication over various transport channels (network sockets, serial ports, etc.). The protocol uses framing, checksums, and sequence numbers to ensure message integrity and ordering regardless of the underlying transport medium.

## Frame Format

Each message is transmitted as a binary frame with the following structure:

```
[STX] [MAGIC] [VER] [LEN] [TYPE] [SEQ] [FLAGS] [PAYLOAD] [CRC32] [ETX]
  1      2      1     2     1     2     1       LEN       4       1
```

### Frame Fields

| Field | Size | Type | Description |
|-------|------|------|-------------|
| **STX** | 1 byte | uint8 | Start of transmission marker (0x02). Marks the beginning of a frame. |
| **MAGIC** | 2 bytes | ascii | Protocol identifier ("TW" = 0x54 0x57). Used to identify Termwire frames. |
| **VER** | 1 byte | uint8 | Protocol version (currently 0x01). Incremented on protocol changes. |
| **LEN** | 2 bytes | uint16 | Payload length in bytes. Range: 0-65,535. Big-endian byte order. |
| **TYPE** | 1 byte | uint8 | Message type. Determines how to interpret the payload. |
| **SEQ** | 2 bytes | uint16 | Sequence number. Used for message ordering and duplicate detection. Range: 0-65,535. Big-endian byte order. |
| **FLAGS** | 1 byte | uint8 | Control flags (reserved for future use). Currently not used but required for alignment. |
| **PAYLOAD** | Variable | bytes | Message data. Length is specified by the LEN field. Can be empty (LEN=0). |
| **CRC32** | 4 bytes | uint32 | Cyclic redundancy check (IEEE 32-bit). Calculated over Version, Type, Sequence, and Payload. |
| **ETX** | 1 byte | uint8 | End of transmission marker (0x03). Marks the end of a frame. |

## Message Types

| Type | Value | Purpose |
|------|-------|---------|
| **PING** | 1 | Keep-alive request. Receiver should respond with a PONG. |
| **PONG** | 2 | Keep-alive response to a PING. |
| **DATA** | 3 | Application data. Payload contains serialized protobuf messages. |
| **CLOSE** | 4 | Connection close notification. Indicates intentional connection termination. |

## Payload Structure

The payload is a variable-length byte sequence that typically contains serialized Protobuf messages. The specific message type and structure are determined by the application layer based on business requirements.

Example use cases:
- For `MESSAGE_TYPE_DATA`: Payload contains protobuf-encoded application messages (e.g., transaction requests, status queries, etc.)
- For `MESSAGE_TYPE_PING`/`MESSAGE_TYPE_PONG`: Payload is typically empty (LEN=0).
- For `MESSAGE_TYPE_CLOSE`: Payload may contain a close reason code or be empty.

> **Note**: The actual Protobuf message definitions are defined separately in `termwire.proto` and are subject to application requirements.

## Frame Constraints

- **Minimum frame size** (without payload): 15 bytes
  ```
  STX(1) + MAGIC(2) + VER(1) + LEN(2) + TYPE(1) + SEQ(2) + FLAGS(1) + CRC32(4) + ETX(1) = 15 bytes
  ```

- **Maximum payload size**: 65,535 bytes (2-byte length field)
- **Maximum frame size**: 65,550 bytes

## Byte Order

- Multi-byte fields use **big-endian (network byte order)** for consistency with network protocols.
- LEN: `data[4:5]` → `(data[4] << 8) | data[5]`
- SEQ: `data[7:8]` → `(data[7] << 8) | data[8]`
- CRC32: `data[crcStart:crcStart+3]` → 4 bytes in big-endian order

## CRC32 Calculation

The CRC32 is calculated over the following fields in order:
```
[VERSION] [TYPE] [SEQUENCE (2 bytes)] [PAYLOAD]
```

The CRC is computed using the IEEE 32-bit polynomial and is stored in big-endian format.

## Validation Rules

A received frame is considered valid if:
1. Length >= 15 bytes (minimum frame size)
2. First byte (STX) equals 0x02
3. Bytes 1-2 (MAGIC) equal "TW" (0x54 0x57)
4. Version byte matches the protocol version
5. Last byte (ETX) equals 0x03
6. CRC32 matches the calculated checksum
7. Payload length matches the LEN field

## Examples

### Example 1: Empty PING Frame

```
[0x02] [0x54 0x57] [0x01] [0x00 0x00] [0x01] [0x00 0x01] [0x00] [CRC32(4)] [0x03]
STX     MAGIC       VER    LEN(0)      TYPE   SEQ        FLAGS  PAYLOAD    ETX
```

### Example 2: DATA Frame with 10-byte Payload

```
[0x02] [0x54 0x57] [0x01] [0x00 0x0A] [0x03] [0x00 0x02] [0x00] [Payload(10)] [CRC32(4)] [0x03]
STX     MAGIC       VER    LEN(10)     TYPE   SEQ        FLAGS  PAYLOAD      CRC32     ETX
```

## Development Notes

- Implementers should always validate frames before processing.
- Sequence numbers should be incremented for each outgoing message to track ordering.
- CRC32 validation ensures payload integrity; frames with invalid CRC should be discarded.
- The MAGIC field provides early frame detection; non-matching frames should be skipped.
- The FLAGS field is reserved for future protocol extensions.
