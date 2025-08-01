package avacadovnc

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/bigangryrobot/avacadovnc/logger"
)

// Server represents a VNC server that listens for and manages incoming client connections.
type Server struct {
	listener net.Listener
	config   *ServerConfig
}

// NewServer creates a new VNC server with the given configuration.
func NewServer(cfg *ServerConfig) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("server config cannot be nil")
	}
	// Initialize the quit channel for graceful shutdown.
	cfg.quit = make(chan struct{})
	return &Server{config: cfg}, nil
}

// Start begins listening for incoming client connections on the specified address.
// This function blocks until the server is stopped.
func (s *Server) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start listener on %s: %w", addr, err)
	}
	s.listener = ln
	logger.Infof("VNC server listening on %s", addr)

	// The main accept loop.
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Check if the listener was closed intentionally.
			select {
			case <-s.config.quit:
				logger.Info("VNC server shutting down.")
				return nil
			default:
				// An unexpected error occurred.
				return fmt.Errorf("failed to accept connection: %w", err)
			}
		}
		// Handle each new connection in its own goroutine.
		go s.handleConnection(conn)
	}
}

// Stop gracefully shuts down the server by closing the listener and signaling all
// active connections to terminate.
func (s *Server) Stop() {
	// Signal shutdown to the Start loop and all connections.
	close(s.config.quit)
	if s.listener != nil {
		// Closing the listener will cause the Accept() call in Start() to return an error.
		s.listener.Close()
	}
}

// handleConnection manages the entire lifecycle of a single client connection.
func (s *Server) handleConnection(conn net.Conn) {
	serverConn, err := NewServerConn(conn, s.config)
	if err != nil {
		logger.Errorf("failed to create server connection for %s: %v", conn.RemoteAddr(), err)
		conn.Close()
		return
	}
	defer serverConn.Close()

	logger.Infof("client connected: %s", conn.RemoteAddr())

	// Execute the handshake process.
	if len(s.config.Handlers) == 0 {
		logger.Errorf("no server handlers configured for client %s", conn.RemoteAddr())
		return
	}
	for _, h := range s.config.Handlers {
		if err := h.Handle(serverConn); err != nil {
			logger.Errorf("handshake failed for client %s: %v", conn.RemoteAddr(), err)
			return
		}
	}

	// The connection is now established. Wait for it to be closed either by the
	// client or by a server shutdown.
	serverConn.Wait()
	logger.Infof("client disconnected: %s", conn.RemoteAddr())
}

// ServerConn represents a server-side VNC connection. It implements the Conn interface.
type ServerConn struct {
	c        net.Conn
	br       *bufio.Reader
	bw       *bufio.Writer
	cfg      *ServerConfig
	protocol string

	colorMap    ColorMap
	desktopName []byte
	encodings   []Encoding

	securityHandler SecurityHandler

	fbHeight uint16
	fbWidth  uint16

	pixelFormat PixelFormat

	quit   chan struct{}
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

// NewServerConn creates a new server-side connection object.
func NewServerConn(c net.Conn, cfg *ServerConfig) (*ServerConn, error) {
	return &ServerConn{
		c:           c,
		cfg:         cfg,
		br:          bufio.NewReader(c),
		bw:          bufio.NewWriter(c),
		pixelFormat: cfg.PixelFormat,
		fbWidth:     cfg.Width,
		fbHeight:    cfg.Height,
		desktopName: []byte(cfg.DesktopName),
		encodings:   cfg.Encodings,
		quit:        cfg.quit, // Use the server's quit channel.
	}, nil
}

// GetEncInstance returns the encoding instance for a given encoding type.
func (sc *ServerConn) GetEncInstance(typ EncodingType) Encoding {
	for _, enc := range sc.encodings {
		if enc.Type() == typ {
			return enc
		}
	}
	return nil
}

// SetEncodings is a client-to-server operation. The server does not send this message.
func (sc *ServerConn) SetEncodings(encs []EncodingType) error {
	return errors.New("server cannot set encodings; this is a client-side message")
}

// ResetAllEncodings resets the state of all supported encodings.
func (sc *ServerConn) ResetAllEncodings() {
	for _, enc := range sc.encodings {
		enc.Reset()
	}
}

// Wait blocks until the connection is closed.
func (sc *ServerConn) Wait() { sc.wg.Wait() }

// Conn returns the underlying network connection.
func (sc *ServerConn) Conn() net.Conn { return sc.c }

// Read reads data from the connection.
func (sc *ServerConn) Read(buf []byte) (int, error) { return sc.br.Read(buf) }

// Write writes data to the connection.
func (sc *ServerConn) Write(buf []byte) (int, error) { return sc.bw.Write(buf) }

// Flush writes buffered data to the network.
func (sc *ServerConn) Flush() error { return sc.bw.Flush() }

// Close closes the connection and cleans up resources.
func (sc *ServerConn) Close() error {
	sc.mu.Lock()
	if sc.closed {
		sc.mu.Unlock()
		return nil
	}
	sc.closed = true
	sc.mu.Unlock()
	return sc.c.Close()
}

// ColorMap returns the connection's color map.
func (sc *ServerConn) ColorMap() ColorMap { return sc.colorMap }

// SetColorMap sets the connection's color map.
func (sc *ServerConn) SetColorMap(cm ColorMap) { sc.colorMap = cm }

// DesktopName returns the server's desktop name.
func (sc *ServerConn) DesktopName() []byte { return sc.desktopName }

// SetDesktopName is a no-op on the server side, as the server defines the name.
func (sc *ServerConn) SetDesktopName(name []byte) {}

// Encodings returns the server's supported encodings.
func (sc *ServerConn) Encodings() []Encoding { return sc.encodings }

// PixelFormat returns the server's pixel format.
func (sc *ServerConn) PixelFormat() PixelFormat { return sc.pixelFormat }

// SetPixelFormat sets the client's desired pixel format.
func (sc *ServerConn) SetPixelFormat(pf PixelFormat) error {
	sc.pixelFormat = pf
	return nil
}

// Protocol returns the negotiated protocol version.
func (sc *ServerConn) Protocol() string { return sc.protocol }

// SetProtoVersion sets the protocol version.
func (sc *ServerConn) SetProtoVersion(pv string) { sc.protocol = pv }

// SecurityHandler returns the negotiated security handler.
func (sc *ServerConn) SecurityHandler() SecurityHandler { return sc.securityHandler }

// SetSecurityHandler sets the security handler.
func (sc *ServerConn) SetSecurityHandler(sh SecurityHandler) error {
	sc.securityHandler = sh
	return nil
}

// Width returns the framebuffer width.
func (sc *ServerConn) Width() uint16 { return sc.fbWidth }

// SetWidth is a no-op on the server side, as the server defines the width.
func (sc *ServerConn) SetWidth(width uint16) {}

// Height returns the framebuffer height.
func (sc *ServerConn) Height() uint16 { return sc.fbHeight }

// SetHeight is a no-op on the server side, as the server defines the height.
func (sc *ServerConn) SetHeight(height uint16) {}

// Config returns the server's configuration.
func (sc *ServerConn) Config() interface{} { return sc.cfg }
