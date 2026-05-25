// Package ecr implements the Termwire ECR (Electronic Cash Register) TCP client.
//
// The package provides two layers of abstraction:
//
//   - [Client] — low-level connection management (connect, disconnect, send/receive frames).
//   - [API] — mid-level typed message helpers (InsertCard, ProcessPayment, GetXReport, …).
//   - [PaymentFlow] — high-level orchestration of the multi-step card payment flow,
//     including optional loyalty point inquiry and confirmation.
//
// # Typical usage
//
//	api := ecr.NewAPI(ecr.DefaultConfig())
//	if err := api.Connect(); err != nil { log.Fatal(err) }
//	defer api.Disconnect()
//
//	flow := ecr.NewPaymentFlow(api)
//	result, err := flow.ExecuteSimplePayment("TX-001", 10000, "TRY")
//
// # Retries and idempotency
//
// [Client.SendMessage] retries up to [Config.MaxRetries] times on transient
// network errors. The original message_id is preserved across retries so the
// POS server can return a cached idempotent response.
package ecr

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
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
	logger    *slog.Logger
}

// NewClient creates a new ECR client
func NewClient(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: config.LogLevel}))
	return &Client{
		config:    config,
		sequence:  0,
		connected: false,
		logger:    logger,
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
	c.logDebug("connecting to POS terminal", slog.String("addr", addr))

	var (
		conn net.Conn
		err  error
	)

	if c.config.TLSEnabled {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS13,
			ServerName: c.config.TLSServerName,
		}
		if tlsCfg.ServerName == "" {
			tlsCfg.ServerName = c.config.POSHost
		}

		if c.config.TLSCAFile != "" {
			caPEM, readErr := os.ReadFile(c.config.TLSCAFile)
			if readErr != nil {
				return fmt.Errorf("failed to read CA file: %w", readErr)
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caPEM) {
				return fmt.Errorf("failed to parse CA certificate")
			}
			tlsCfg.RootCAs = caPool
		}

		if c.config.TLSCertFile != "" && c.config.TLSKeyFile != "" {
			cert, certErr := tls.LoadX509KeyPair(c.config.TLSCertFile, c.config.TLSKeyFile)
			if certErr != nil {
				return fmt.Errorf("failed to load client TLS cert/key: %w", certErr)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}

		dialer := &tls.Dialer{
			NetDialer: &net.Dialer{Timeout: c.config.ConnectTimeout},
			Config:    tlsCfg,
		}
		conn, err = dialer.Dial("tcp", addr)
	} else {
		conn, err = net.DialTimeout("tcp", addr, c.config.ConnectTimeout)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to POS: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)
	c.connected = true

	c.logDebug("connected to POS terminal successfully")

	return nil
}

// Disconnect closes the connection to the POS terminal
func (c *Client) Disconnect() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if !c.connected {
		return nil
	}

	c.logDebug("disconnecting from POS terminal")

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

	c.logDebug("disconnected from POS terminal")

	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return c.connected
}

// SendMessage sends a message to the POS and waits for response.
// Retries up to config.MaxRetries times on transient network errors, preserving the
// original message_id so the server can return a cached idempotent response.
func (c *Client) SendMessage(msg *pb.Message) (*pb.Message, error) {
	c.connMutex.RLock()
	if !c.connected {
		c.connMutex.RUnlock()
		return nil, fmt.Errorf("not connected to POS")
	}
	c.connMutex.RUnlock()

	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	frame := protocol.NewFrame(protocol.FRAME_TYPE_DATA, c.getNextSequence(), data)

	c.logDebug("sending message", slog.String("message_id", msg.MessageId))

	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			c.logDebug("retry", slog.Int("attempt", attempt), slog.Int("max", c.config.MaxRetries), slog.String("message_id", msg.MessageId))
			time.Sleep(c.config.RetryDelay)
		}

		if err := c.writeFrame(frame); err != nil {
			lastErr = fmt.Errorf("failed to send frame: %w", err)
			continue
		}

		responseFrame, err := c.readFrame()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		response := &pb.Message{}
		if err := proto.Unmarshal(responseFrame.Payload, response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		c.logDebug("received response", slog.String("message_id", response.MessageId))
		return response, nil
	}

	return nil, lastErr
}

// Ping sends a ping to the POS terminal
func (c *Client) Ping() error {
	c.connMutex.RLock()
	if !c.connected {
		c.connMutex.RUnlock()
		return fmt.Errorf("not connected to POS")
	}
	c.connMutex.RUnlock()

	c.logDebug("sending PING")

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

	c.logDebug("received PONG")

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
	return protocol.ReadFrame(c.reader)
}

// getNextSequence returns the next sequence number
func (c *Client) getNextSequence() uint16 {
	c.seqMutex.Lock()
	defer c.seqMutex.Unlock()
	c.sequence++
	return c.sequence
}

// logDebug logs a debug message if debug is enabled
func (c *Client) logDebug(msg string, args ...any) {
	if c.config.Debug {
		c.logger.Debug(msg, args...)
	}
}

// GetConfig returns the client configuration
func (c *Client) GetConfig() *Config {
	return c.config
}
