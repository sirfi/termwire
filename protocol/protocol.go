// Package protocol implements the Termwire binary frame codec and contains
// the generated Protobuf message definitions.
//
// # Wire Frame Layout
//
//	[STX 1B][MAGIC 2B='TW'][VER 1B][LEN 2B][TYPE 1B][SEQ 2B][FLAGS 1B][PAYLOAD NB][CRC32 4B][ETX 1B]
//
// CRC32 (IEEE) is computed over: Version, Type, Sequence (2 bytes), Payload.
// Multi-byte integer fields use big-endian byte order.
//
// Use [NewFrame] to construct frames, [ParseFrame] or [ReadFrame] to decode them,
// and [Frame.Serialize] to encode them for transmission.
//
//go:generate protoc --go_out=. --go_opt=paths=source_relative --proto_path=. termwire.proto

package protocol

import (
	"bufio"
	"fmt"
	"hash/crc32"
	"io"
)

const NAME = "termwire"
const VERSION = 1
const MAGIC = "TW"
const STX = 0x02
const ETX = 0x03

type FrameType uint8

const (
	FRAME_TYPE_PING FrameType = iota + 1
	FRAME_TYPE_PONG
	FRAME_TYPE_DATA
	FRAME_TYPE_CLOSE
)

type Frame struct {
	Version  uint8
	Type     uint8
	Sequence uint16
	Payload  []byte
	CRC32    uint32
}

func (m *Frame) IsValid() bool {
	if m.Version != VERSION {
		return false
	}
	if m.Type < uint8(FRAME_TYPE_PING) || m.Type > uint8(FRAME_TYPE_CLOSE) {
		return false
	}
	calculatedCRC := m.CalculateCRC32()
	if m.CRC32 != calculatedCRC {
		return false
	}
	return true
}

func (m *Frame) CalculateCRC32() uint32 {
	buf := make([]byte, 4+len(m.Payload))
	buf[0] = m.Version
	buf[1] = m.Type
	buf[2] = byte(m.Sequence >> 8)
	buf[3] = byte(m.Sequence)
	copy(buf[4:], m.Payload)

	return crc32.ChecksumIEEE(buf)
}

func NewFrame(msgType FrameType, sequence uint16, payload []byte) *Frame {
	m := &Frame{
		Version:  VERSION,
		Type:     uint8(msgType),
		Sequence: sequence,
		Payload:  payload,
	}
	m.CRC32 = m.CalculateCRC32()
	return m
}

func CalculateLength(payloadLen int) int {
	/*
		[STX]      1 byte   0x02
		[MAGIC]    2 byte   0x54 0x57   ('T','W')
		[VER]      1 byte   0x01
		[LEN]      2 byte   uint16 (payload length)
		[TYPE]     1 byte
		[SEQ]      2 byte   uint16
		[FLAGS]    1 byte
		[PAYLOAD]  LEN byte
		[CRC32]    4 byte   uint32
		[ETX]      1 byte   0x03
	*/
	return /*STX*/ 1 + /*MAGIC*/ 2 + /*VER*/ 1 + /*LEN*/ 2 + /*TYPE*/ 1 + /*SEQ*/ 2 + /*FLAGS*/ 1 + payloadLen + /*CRC32*/ 4 + /*ETX*/ 1
}

func (m *Frame) Serialize() []byte {
	payloadLen := len(m.Payload)
	totalLen := CalculateLength(payloadLen)
	data := make([]byte, totalLen)
	data[0] = STX
	copy(data[1:3], []byte(MAGIC))
	data[3] = m.Version
	data[4] = byte(payloadLen >> 8)
	data[5] = byte(payloadLen)
	data[6] = m.Type
	data[7] = byte(m.Sequence >> 8)
	data[8] = byte(m.Sequence)
	data[9] = 0 // FLAGS - reserved, always 0
	if payloadLen > 0 {
		copy(data[10:10+payloadLen], m.Payload)
	}
	crcStart := 10 + payloadLen
	data[crcStart] = byte(m.CRC32 >> 24)
	data[crcStart+1] = byte(m.CRC32 >> 16)
	data[crcStart+2] = byte(m.CRC32 >> 8)
	data[crcStart+3] = byte(m.CRC32)
	data[totalLen-1] = ETX
	return data
}

func ParseFrame(data []byte) *Frame {
	minLen := CalculateLength(0)
	if len(data) < minLen {
		return nil
	}

	if data[0] != STX || string(data[1:3]) != MAGIC {
		return nil
	}

	payloadLen := int(data[4])<<8 | int(data[5])
	totalLen := CalculateLength(payloadLen)
	if len(data) < totalLen || data[totalLen-1] != ETX {
		return nil
	}

	m := &Frame{
		Version:  data[3],
		Type:     data[6],
		Sequence: uint16(data[7])<<8 | uint16(data[8]),
	}

	if payloadLen > 0 {
		m.Payload = make([]byte, payloadLen)
		copy(m.Payload, data[10:10+payloadLen])
	}

	crcStart := 10 + payloadLen
	m.CRC32 = uint32(data[crcStart])<<24 | uint32(data[crcStart+1])<<16 | uint32(data[crcStart+2])<<8 | uint32(data[crcStart+3])

	return m
}

// ReadFrame reads and validates a single frame from a buffered reader.
// This is the canonical implementation shared by both client and server.
func ReadFrame(reader *bufio.Reader) (*Frame, error) {
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == STX {
			break
		}
	}

	magic := make([]byte, 2)
	if _, err := io.ReadFull(reader, magic); err != nil {
		return nil, err
	}
	if string(magic) != MAGIC {
		return nil, fmt.Errorf("invalid magic bytes: %v", magic)
	}

	version, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	lengthBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, lengthBytes); err != nil {
		return nil, err
	}
	payloadLen := int(lengthBytes[0])<<8 | int(lengthBytes[1])

	frameType, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	seqBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, seqBytes); err != nil {
		return nil, err
	}
	sequence := uint16(seqBytes[0])<<8 | uint16(seqBytes[1])

	if _, err = reader.ReadByte(); err != nil { // FLAGS — reserved
		return nil, err
	}

	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, err
		}
	}

	crcBytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, crcBytes); err != nil {
		return nil, err
	}
	crc32Value := uint32(crcBytes[0])<<24 | uint32(crcBytes[1])<<16 | uint32(crcBytes[2])<<8 | uint32(crcBytes[3])

	etx, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if etx != ETX {
		return nil, fmt.Errorf("invalid ETX byte: 0x%02x", etx)
	}

	frame := &Frame{
		Version:  version,
		Type:     frameType,
		Sequence: sequence,
		Payload:  payload,
		CRC32:    crc32Value,
	}

	if !frame.IsValid() {
		return nil, fmt.Errorf("invalid frame: CRC mismatch")
	}

	return frame, nil
}
