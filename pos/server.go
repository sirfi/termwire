package pos

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
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
	logger      *slog.Logger
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
func NewServer(config *Config) (*Server, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: config.LogLevel}))
	handler, err := NewMessageHandler(config, logger)
	if err != nil {
		return nil, fmt.Errorf("creating message handler: %w", err)
	}
	return &Server{
		config:      config,
		handler:     handler,
		connections: make(map[string]*ClientConnection),
		shutdown:    make(chan struct{}),
		logger:      logger,
	}, nil
}

// Start starts the TCP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.logger.Info("starting POS server", slog.String("addr", addr), slog.Bool("tls", s.config.TLSEnabled))

	var (
		listener net.Listener
		err      error
	)

	if s.config.TLSEnabled {
		cert, err := tls.LoadX509KeyPair(s.config.TLSCertFile, s.config.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS cert/key: %w", err)
		}

		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		}

		if s.config.TLSCAFile != "" {
			caPEM, err := os.ReadFile(s.config.TLSCAFile)
			if err != nil {
				return fmt.Errorf("failed to read CA file: %w", err)
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caPEM) {
				return fmt.Errorf("failed to parse CA certificate")
			}
			tlsCfg.ClientCAs = caPool
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		}

		listener, err = tls.Listen("tcp", addr, tlsCfg)
	} else {
		listener, err = net.Listen("tcp", addr)
	}

	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	s.listener = listener
	s.logger.Info("POS server started", slog.String("addr", addr))

	s.wg.Add(1)
	go s.acceptConnections()

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
			s.logger.Info("accept loop stopped")
			return
		default:
		}

		if dl, ok := s.listener.(interface{ SetDeadline(t time.Time) error }); ok {
			dl.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-s.shutdown:
				return
			default:
				s.logger.Error("accept error", slog.Any("error", err))
				continue
			}
		}

		s.logger.Info("new connection", slog.String("remote_addr", conn.RemoteAddr().String()))

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

	s.connMutex.Lock()
	s.connections[clientID] = clientConn
	s.connMutex.Unlock()

	s.logger.Info("client connected",
		slog.String("client_id", clientID),
		slog.String("remote_addr", conn.RemoteAddr().String()),
	)

	s.handleClientMessages(clientConn)

	s.connMutex.Lock()
	delete(s.connections, clientID)
	s.connMutex.Unlock()

	conn.Close()
	s.logger.Info("client disconnected", slog.String("client_id", clientID))
}

// handleClientMessages handles messages from a client
func (s *Server) handleClientMessages(client *ClientConnection) {
	for {
		select {
		case <-s.shutdown:
			return
		default:
		}

		client.conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))

		frame, err := s.readFrame(client.reader)
		if err != nil {
			if err == io.EOF {
				s.logger.Info("client disconnected (EOF)", slog.String("client_id", client.id))
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				s.logger.Warn("read timeout", slog.String("client_id", client.id))
			} else {
				s.logger.Error("frame read error",
					slog.String("client_id", client.id),
					slog.Any("error", err),
				)
			}
			return
		}

		client.lastActive = time.Now()

		if err := s.handleFrame(client, frame); err != nil {
			if err == io.EOF {
				s.logger.Info("client disconnected (EOF)", slog.String("client_id", client.id))
				return
			}
			s.logger.Error("frame handle error", slog.Any("error", err))
			return
		}
	}
}

// readFrame reads a complete frame from the connection
func (s *Server) readFrame(reader *bufio.Reader) (*protocol.Frame, error) {
	return protocol.ReadFrame(reader)
}

// handleFrame processes a received frame
func (s *Server) handleFrame(client *ClientConnection, frame *protocol.Frame) error {
	s.logger.Debug("received frame",
		slog.Int("type", int(frame.Type)),
		slog.Int("sequence", int(frame.Sequence)),
		slog.Int("payload_len", len(frame.Payload)),
	)

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
	s.logger.Debug("received PING", slog.String("client_id", client.id))
	pongFrame := protocol.NewFrame(protocol.FRAME_TYPE_PONG, frame.Sequence, nil)
	return s.sendFrame(client, pongFrame)
}

// handleData handles a data frame
func (s *Server) handleData(client *ClientConnection, frame *protocol.Frame) error {
	msg := &pb.Message{}
	if err := proto.Unmarshal(frame.Payload, msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	s.logger.Info("received message",
		slog.String("message_id", msg.MessageId),
		slog.String("client_id", client.id),
	)

	response, err := s.handler.HandleMessage(msg)
	if err != nil {
		s.logger.Error("message handle error",
			slog.String("message_id", msg.MessageId),
			slog.Any("error", err),
		)
		return s.sendErrorResponse(client, frame.Sequence, err.Error())
	}

	responseData, err := proto.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	client.seqMutex.Lock()
	client.sequence++
	responseSeq := client.sequence
	client.seqMutex.Unlock()

	responseFrame := protocol.NewFrame(protocol.FRAME_TYPE_DATA, responseSeq, responseData)
	return s.sendFrame(client, responseFrame)
}

// handleClose handles a close frame
func (s *Server) handleClose(client *ClientConnection, frame *protocol.Frame) error {
	s.logger.Info("received CLOSE", slog.String("client_id", client.id))
	return io.EOF
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

	s.logger.Debug("sent frame",
		slog.Int("type", int(frame.Type)),
		slog.Int("sequence", int(frame.Sequence)),
		slog.Int("payload_len", len(frame.Payload)),
	)

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

	errorData, err := proto.Marshal(errorResponse)
	if err != nil {
		return err
	}

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
			s.logger.Info("closing idle connection", slog.String("client_id", id))
			client.conn.Close()
			delete(s.connections, id)
		}
	}
}

// Stop stops the server gracefully
func (s *Server) Stop() {
	s.logger.Info("stopping POS server")

	close(s.shutdown)

	if s.listener != nil {
		s.listener.Close()
	}

	s.connMutex.Lock()
	for _, client := range s.connections {
		client.conn.Close()
	}
	s.connMutex.Unlock()

	s.wg.Wait()

	if s.handler != nil {
		s.handler.Close()
	}

	s.logger.Info("POS server stopped")
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
