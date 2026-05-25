package pos

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

// Server represents the POS TCP server
type Server struct {
	config      *Config
	handler     *MessageHandler
	listener    net.Listener
	connections map[string]*ClientConnection
	connMutex   sync.RWMutex
	shutdown    chan struct{}
	wg          sync.WaitGroup
}

// ClientConnection represents a connected ECR client
type ClientConnection struct {
	conn       net.Conn
	id         string
	sequence   uint16
	seqMutex   sync.Mutex
	lastActive time.Time
	reader     *bufio.Reader
	writer     *bufio.Writer
}

// NewServer creates a new POS server
func NewServer(config *Config) *Server {
	return &Server{
		config:      config,
		handler:     NewMessageHandler(config),
		connections: make(map[string]*ClientConnection),
		shutdown:    make(chan struct{}),
	}
}

// Start starts the TCP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("[SERVER] Starting POS server on %s", addr)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	s.listener = listener
	log.Printf("[SERVER] POS server started successfully on %s", addr)

	// Start accepting connections
	s.wg.Add(1)
	go s.acceptConnections()

	// Start connection monitor
	s.wg.Add(1)
	go s.monitorConnections()

	return nil
}

// acceptConnections accepts incoming client connections
func (s *Server) acceptConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.shutdown:
			log.Println("[SERVER] Stopping accept loop")
			return
		default:
		}

		// Set accept deadline to allow checking shutdown channel
		s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))

		conn, err := s.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-s.shutdown:
				return
			default:
				log.Printf("[SERVER] Error accepting connection: %v", err)
				continue
			}
		}

		log.Printf("[SERVER] New connection from %s", conn.RemoteAddr())

		// Handle the connection
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()

	clientID := fmt.Sprintf("client-%s-%d", conn.RemoteAddr(), time.Now().Unix())

	clientConn := &ClientConnection{
		conn:       conn,
		id:         clientID,
		sequence:   0,
		lastActive: time.Now(),
		reader:     bufio.NewReader(conn),
		writer:     bufio.NewWriter(conn),
	}

	// Register connection
	s.connMutex.Lock()
	s.connections[clientID] = clientConn
	s.connMutex.Unlock()

	log.Printf("[SERVER] Client connected: %s (%s)", clientID, conn.RemoteAddr())

	// Handle client messages
	s.handleClientMessages(clientConn)

	// Cleanup on disconnect
	s.connMutex.Lock()
	delete(s.connections, clientID)
	s.connMutex.Unlock()

	conn.Close()
	log.Printf("[SERVER] Client disconnected: %s", clientID)
}

// handleClientMessages handles messages from a client
func (s *Server) handleClientMessages(client *ClientConnection) {
	for {
		select {
		case <-s.shutdown:
			return
		default:
		}

		// Set read deadline
		client.conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))

		// Read frame
		frame, err := s.readFrame(client.reader)
		if err != nil {
			if err == io.EOF {
				log.Printf("[SERVER] Client %s disconnected", client.id)
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("[SERVER] Read timeout for client %s", client.id)
			} else {
				log.Printf("[SERVER] Error reading frame from client %s: %v", client.id, err)
			}
			return
		}

		client.lastActive = time.Now()

		// Handle frame
		if err := s.handleFrame(client, frame); err != nil {
			log.Printf("[SERVER] Error handling frame: %v", err)
			// Send error response and continue
			continue
		}
	}
}

// readFrame reads a complete frame from the connection
func (s *Server) readFrame(reader *bufio.Reader) (*protocol.Frame, error) {
	// Read until we find STX
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == protocol.STX {
			break
		}
	}

	// Read MAGIC
	magic := make([]byte, 2)
	if _, err := io.ReadFull(reader, magic); err != nil {
		return nil, err
	}
	if string(magic) != protocol.MAGIC {
		return nil, fmt.Errorf("invalid magic bytes: %v", magic)
	}

	// Read version
	version, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	// Read length (2 bytes)
	lengthBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, lengthBytes); err != nil {
		return nil, err
	}
	payloadLen := int(lengthBytes[0])<<8 | int(lengthBytes[1])

	// Read type
	frameType, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	// Read sequence (2 bytes)
	seqBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, seqBytes); err != nil {
		return nil, err
	}
	sequence := uint16(seqBytes[0])<<8 | uint16(seqBytes[1])

	// Read flags
	_, err = reader.ReadByte()
	if err != nil {
		return nil, err
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, err
		}
	}

	// Read CRC32 (4 bytes)
	crcBytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, crcBytes); err != nil {
		return nil, err
	}
	crc32Value := uint32(crcBytes[0])<<24 | uint32(crcBytes[1])<<16 |
		uint32(crcBytes[2])<<8 | uint32(crcBytes[3])

	// Read ETX
	etx, err := reader.ReadByte()
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

// handleFrame processes a received frame
func (s *Server) handleFrame(client *ClientConnection, frame *protocol.Frame) error {
	log.Printf("[SERVER] Received frame - Type: %d, Sequence: %d, PayloadLen: %d",
		frame.Type, frame.Sequence, len(frame.Payload))

	switch protocol.FrameType(frame.Type) {
	case protocol.FRAME_TYPE_PING:
		return s.handlePing(client, frame)
	case protocol.FRAME_TYPE_DATA:
		return s.handleData(client, frame)
	case protocol.FRAME_TYPE_CLOSE:
		return s.handleClose(client, frame)
	default:
		return fmt.Errorf("unknown frame type: %d", frame.Type)
	}
}

// handlePing handles a ping frame
func (s *Server) handlePing(client *ClientConnection, frame *protocol.Frame) error {
	log.Printf("[SERVER] Received PING from client %s", client.id)

	// Send PONG response
	pongFrame := protocol.NewFrame(protocol.FRAME_TYPE_PONG, frame.Sequence, nil)
	return s.sendFrame(client, pongFrame)
}

// handleData handles a data frame
func (s *Server) handleData(client *ClientConnection, frame *protocol.Frame) error {
	// Parse protobuf message
	msg := &pb.Message{}
	if err := proto.Unmarshal(frame.Payload, msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	log.Printf("[SERVER] Received message - ID: %s, Type: %T", msg.MessageId, msg.Body)

	// Handle the message
	response, err := s.handler.HandleMessage(msg)
	if err != nil {
		log.Printf("[SERVER] Error handling message: %v", err)
		// Send error response
		return s.sendErrorResponse(client, frame.Sequence, err.Error())
	}

	// Marshal response
	responseData, err := proto.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Send response frame
	client.seqMutex.Lock()
	client.sequence++
	responseSeq := client.sequence
	client.seqMutex.Unlock()

	responseFrame := protocol.NewFrame(protocol.FRAME_TYPE_DATA, responseSeq, responseData)
	return s.sendFrame(client, responseFrame)
}

// handleClose handles a close frame
func (s *Server) handleClose(client *ClientConnection, frame *protocol.Frame) error {
	log.Printf("[SERVER] Received CLOSE from client %s", client.id)
	return io.EOF // Signal to close connection
}

// sendFrame sends a frame to the client
func (s *Server) sendFrame(client *ClientConnection, frame *protocol.Frame) error {
	client.conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))

	data := frame.Serialize()

	_, err := client.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write frame: %w", err)
	}

	if err := client.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	log.Printf("[SERVER] Sent frame - Type: %d, Sequence: %d, PayloadLen: %d",
		frame.Type, frame.Sequence, len(frame.Payload))

	return nil
}

// sendErrorResponse sends an error response to the client
func (s *Server) sendErrorResponse(client *ClientConnection, sequence uint16, errorMsg string) error {
	errorResponse := &pb.Message{
		MessageId: fmt.Sprintf("ERR-%d", time.Now().Unix()),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_ErrorResponse{
			ErrorResponse: &pb.ErrorResponse{
				Code:    "99",
				Message: errorMsg,
			},
		},
	}

	// Marshal error message
	errorData, err := proto.Marshal(errorResponse)
	if err != nil {
		return err
	}

	// Send error frame
	errorFrame := protocol.NewFrame(protocol.FRAME_TYPE_DATA, sequence, errorData)
	return s.sendFrame(client, errorFrame)
}

// monitorConnections monitors active connections and closes idle ones
func (s *Server) monitorConnections() {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.checkIdleConnections()
		}
	}
}

// checkIdleConnections closes idle connections
func (s *Server) checkIdleConnections() {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	now := time.Now()
	for id, client := range s.connections {
		if now.Sub(client.lastActive) > s.config.IdleTimeout {
			log.Printf("[SERVER] Closing idle connection: %s", id)
			client.conn.Close()
			delete(s.connections, id)
		}
	}
}

// Stop stops the server gracefully
func (s *Server) Stop() {
	log.Println("[SERVER] Stopping POS server...")

	close(s.shutdown)

	if s.listener != nil {
		s.listener.Close()
	}

	// Close all active connections
	s.connMutex.Lock()
	for _, client := range s.connections {
		client.conn.Close()
	}
	s.connMutex.Unlock()

	// Wait for all goroutines to finish
	s.wg.Wait()

	log.Println("[SERVER] POS server stopped")
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]interface{} {
	s.connMutex.RLock()
	activeConnections := len(s.connections)
	s.connMutex.RUnlock()

	stats := map[string]interface{}{
		"active_connections": activeConnections,
		"address":            fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		"uptime":             time.Now().Format(time.RFC3339),
	}

	// Add transaction statistics
	if s.handler != nil {
		txnStats := s.handler.GetTransactionManager().GetStatistics()
		for k, v := range txnStats {
			stats["txn_"+k] = v
		}
	}

	return stats
}

// GetHandler returns the message handler
func (s *Server) GetHandler() *MessageHandler {
	return s.handler
}
