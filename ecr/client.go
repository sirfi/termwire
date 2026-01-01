package ecr

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/sirfi/termwire/protocol"
	pb "github.com/sirfi/termwire/protocol"
	"google.golang.org/protobuf/proto"
)

// Client represents an ECR TCP client
type Client struct {
	config    *Config
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	sequence  uint16
	seqMutex  sync.Mutex
	connected bool
	connMutex sync.RWMutex
}

// NewClient creates a new ECR client
func NewClient(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}
	return &Client{
		config:    config,
		sequence:  0,
		connected: false,
	}
}

// Connect establishes a connection to the POS terminal
func (c *Client) Connect() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

	addr := net.JoinHostPort(c.config.POSHost, fmt.Sprintf("%d", c.config.POSPort))
	c.logDebug("Connecting to POS terminal at %s", addr)

	conn, err := net.DialTimeout("tcp", addr, c.config.ConnectTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to POS: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)
	c.connected = true

	c.logDebug("Connected to POS terminal successfully")

	return nil
}

// Disconnect closes the connection to the POS terminal
func (c *Client) Disconnect() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if !c.connected {
		return nil
	}

	c.logDebug("Disconnecting from POS terminal")

	// Send close frame
	if c.conn != nil {
		closeFrame := protocol.NewFrame(protocol.FRAME_TYPE_CLOSE, c.getNextSequence(), nil)
		c.writeFrame(closeFrame)
		c.conn.Close()
	}

	c.connected = false
	c.conn = nil
	c.reader = nil
	c.writer = nil

	c.logDebug("Disconnected from POS terminal")

	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return c.connected
}

// SendMessage sends a message to the POS and waits for response
func (c *Client) SendMessage(msg *pb.Message) (*pb.Message, error) {
	c.connMutex.RLock()
	if !c.connected {
		c.connMutex.RUnlock()
		return nil, fmt.Errorf("not connected to POS")
	}
	c.connMutex.RUnlock()

	// Marshal message
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create frame
	frame := protocol.NewFrame(protocol.FRAME_TYPE_DATA, c.getNextSequence(), data)

	c.logDebug("Sending message - ID: %s, Type: %T", msg.MessageId, msg.Body)

	// Send frame
	if err := c.writeFrame(frame); err != nil {
		return nil, fmt.Errorf("failed to send frame: %w", err)
	}

	// Read response
	responseFrame, err := c.readFrame()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	response := &pb.Message{}
	if err := proto.Unmarshal(responseFrame.Payload, response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.logDebug("Received response - ID: %s, Type: %T", response.MessageId, response.Body)

	return response, nil
}

// Ping sends a ping to the POS terminal
func (c *Client) Ping() error {
	c.connMutex.RLock()
	if !c.connected {
		c.connMutex.RUnlock()
		return fmt.Errorf("not connected to POS")
	}
	c.connMutex.RUnlock()

	c.logDebug("Sending PING")

	pingFrame := protocol.NewFrame(protocol.FRAME_TYPE_PING, c.getNextSequence(), nil)
	if err := c.writeFrame(pingFrame); err != nil {
		return fmt.Errorf("failed to send ping: %w", err)
	}

	// Read PONG response
	pongFrame, err := c.readFrame()
	if err != nil {
		return fmt.Errorf("failed to read pong: %w", err)
	}

	if protocol.FrameType(pongFrame.Type) != protocol.FRAME_TYPE_PONG {
		return fmt.Errorf("expected PONG, got frame type %d", pongFrame.Type)
	}

	c.logDebug("Received PONG")

	return nil
}

// writeFrame writes a frame to the connection
func (c *Client) writeFrame(frame *protocol.Frame) error {
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout)); err != nil {
		return err
	}

	data := frame.Serialize()

	if _, err := c.writer.Write(data); err != nil {
		return err
	}

	if err := c.writer.Flush(); err != nil {
		return err
	}

	return nil
}

// readFrame reads a frame from the connection
func (c *Client) readFrame() (*protocol.Frame, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout)); err != nil {
		return nil, err
	}

	// Read until we find STX
	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == protocol.STX {
			break
		}
	}

	// Read MAGIC
	magic := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, magic); err != nil {
		return nil, err
	}
	if string(magic) != protocol.MAGIC {
		return nil, fmt.Errorf("invalid magic bytes: %v", magic)
	}

	// Read version
	version, err := c.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	// Read length (2 bytes)
	lengthBytes := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, lengthBytes); err != nil {
		return nil, err
	}
	payloadLen := int(lengthBytes[0])<<8 | int(lengthBytes[1])

	// Read type
	frameType, err := c.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	// Read sequence (2 bytes)
	seqBytes := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, seqBytes); err != nil {
		return nil, err
	}
	sequence := uint16(seqBytes[0])<<8 | uint16(seqBytes[1])

	// Read flags
	_, err = c.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(c.reader, payload); err != nil {
			return nil, err
		}
	}

	// Read CRC32 (4 bytes)
	crcBytes := make([]byte, 4)
	if _, err := io.ReadFull(c.reader, crcBytes); err != nil {
		return nil, err
	}
	crc32Value := uint32(crcBytes[0])<<24 | uint32(crcBytes[1])<<16 |
		uint32(crcBytes[2])<<8 | uint32(crcBytes[3])

	// Read ETX
	etx, err := c.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if etx != protocol.ETX {
		return nil, fmt.Errorf("invalid ETX byte: 0x%02x", etx)
	}

	frame := &protocol.Frame{
		Version:  version,
		Type:     frameType,
		Sequence: sequence,
		Payload:  payload,
		CRC32:    crc32Value,
	}

	// Validate frame
	if !frame.IsValid() {
		return nil, fmt.Errorf("invalid frame: CRC mismatch")
	}

	return frame, nil
}

// getNextSequence returns the next sequence number
func (c *Client) getNextSequence() uint16 {
	c.seqMutex.Lock()
	defer c.seqMutex.Unlock()
	c.sequence++
	return c.sequence
}

// logDebug logs a debug message if debug is enabled
func (c *Client) logDebug(format string, args ...interface{}) {
	if c.config.Debug {
		log.Printf("[ECR CLIENT] "+format, args...)
	}
}

// GetConfig returns the client configuration
func (c *Client) GetConfig() *Config {
	return c.config
}
