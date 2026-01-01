package protocol

import (
	"testing"
)

func TestNewFrame(t *testing.T) {
	tests := []struct {
		name        string
		msgType     FrameType
		sequence    uint16
		payload     []byte
		expectValid bool
	}{
		{
			name:        "Ping frame",
			msgType:     FRAME_TYPE_PING,
			sequence:    1,
			payload:     []byte{},
			expectValid: true,
		},
		{
			name:        "Data frame",
			msgType:     FRAME_TYPE_DATA,
			sequence:    2,
			payload:     []byte("test"),
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := NewFrame(tt.msgType, tt.sequence, tt.payload)
			if frame == nil {
				t.Fatal("NewFrame returned nil")
			}
			if frame.Version != VERSION {
				t.Errorf("Version: got %d, want %d", frame.Version, VERSION)
			}
			if frame.Type != uint8(tt.msgType) {
				t.Errorf("Type: got %d, want %d", frame.Type, uint8(tt.msgType))
			}
			if !frame.IsValid() {
				t.Error("Frame should be valid")
			}
		})
	}
}

func TestFrameIsValid(t *testing.T) {
	frame := NewFrame(FRAME_TYPE_PING, 1, []byte{})
	if !frame.IsValid() {
		t.Error("Valid frame should pass IsValid")
	}

	frame.Version = 99
	if frame.IsValid() {
		t.Error("Invalid version should fail IsValid")
	}
}

func TestCalculateCRC32(t *testing.T) {
	frame := NewFrame(FRAME_TYPE_DATA, 42, []byte("payload"))
	crc1 := frame.CalculateCRC32()
	crc2 := frame.CalculateCRC32()
	if crc1 != crc2 {
		t.Error("CRC32 should be consistent")
	}
}

func TestCalculateLength(t *testing.T) {
	tests := []struct {
		payloadLen int
		expected   int
	}{
		{0, 15},
		{10, 25},
		{256, 271},
	}

	for _, tt := range tests {
		result := CalculateLength(tt.payloadLen)
		if result != tt.expected {
			t.Errorf("CalculateLength(%d): got %d, want %d", tt.payloadLen, result, tt.expected)
		}
	}
}
