package vnc2video

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/bigangryrobot/vnc2video/logger"
)

var (
	// DefaultClientHandlers is the default set of handlers for the VNC client handshake.
	// These handlers are executed in sequence to negotiate the protocol version,
	// security, and initial framebuffer state with the server.
	DefaultClientHandlers = []Handler{
		&DefaultClientVersionHandler{},
		&DefaultClientSecurityHandler{},
		&DefaultClientClientInitHandler{},
		&DefaultClientServerInitHandler{},
		&DefaultClientMessageHandler{},
	}
)

// Connect establishes a connection with a VNC server and performs the initial handshake.
// It takes a context for cancellation, a network connection, and a client configuration.
// On success, it returns a fully initialized ClientConn ready for interaction.
// On failure, it returns an error and ensures the connection is closed.
func Connect(ctx context.Context, c net.Conn, cfg *ClientConfig) (*ClientConn, error) {
	// Set an initial deadline for the handshake process.
	// This prevents a non-responsive server from holding the connection indefinitely.
	c.SetDeadline(time.Now().Add(10 * time.Second))
	defer c.SetDeadline(time.Time{}) // Clear the deadline after the handshake is done.

	conn, err := NewClientConn(c, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client connection: %w", err)
	}

	// Use default handlers if none are provided in the config.
	if len(cfg.Handlers) == 0 {
		cfg.Handlers = DefaultClientHandlers
	}

	// Execute the handshake handlers sequentially.
	for _, h := range cfg.Handlers {
		if err := h.Handle(conn); err != nil {
			conn.Close() // Ensure connection is closed on any handshake failure.
			return nil, fmt.Errorf("handshake failed during handler %T: %w", h, err)
		}
	}

	return conn, nil
}

// ClientConn represents a client connection to a VNC server. It manages all
// state and communication with the server, and implements the Conn interface.
type ClientConn struct {
	c        net.Conn
	br       *bufio.Reader
	bw       *bufio.Writer
	cfg      *ClientConfig
	protocol string

	colorMap    ColorMap
	Canvas      *VncCanvas
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

// NewClientConn creates a new, uninitialized client connection.
func NewClientConn(c net.Conn, cfg *ClientConfig) (*ClientConn, error) {
	if len(cfg.Encodings) == 0 {
		return nil, errors.New("at least one encoding must be specified in the client config")
	}
	return &ClientConn{
		c:           c,
		cfg:         cfg,
		br:          bufio.NewReader(c),
		bw:          bufio.NewWriter(c),
		encodings:   cfg.Encodings,
		pixelFormat: cfg.PixelFormat,
		quit:        make(chan struct{}),
	}, nil
}

// Config returns the client configuration.
func (c *ClientConn) Config() interface{} {
	return c.cfg
}

// GetEncInstance returns the encoding instance for a given encoding type.
func (c *ClientConn) GetEncInstance(typ EncodingType) Encoding {
	for _, enc := range c.encodings {
		if enc.Type() == typ {
			return enc
		}
	}
	return nil
}

// Wait blocks until the connection is fully closed and all goroutines have exited.
func (c *ClientConn) Wait() {
	c.wg.Wait()
}

// Conn returns the underlying network connection.
func (c *ClientConn) Conn() net.Conn {
	return c.c
}

// SetProtoVersion sets the protocol version for the connection.
func (c *ClientConn) SetProtoVersion(pv string) {
	c.protocol = pv
}

// SetEncodings sends a SetEncodings message to the server.
func (c *ClientConn) SetEncodings(encs []EncodingType) error {
	msg := &SetEncodings{
		Encodings: encs,
	}
	return msg.Write(c)
}

// Flush writes any buffered data to the underlying connection.
func (c *ClientConn) Flush() error {
	return c.bw.Flush()
}

// Close gracefully shuts down the client connection, stops all related goroutines,
// and closes the underlying network connection. It is safe to call multiple times.
func (c *ClientConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	close(c.quit)
	err := c.c.Close()
	c.wg.Wait()
	return err
}

// Read reads data from the connection's buffered reader.
func (c *ClientConn) Read(buf []byte) (int, error) {
	return c.br.Read(buf)
}

// Write writes data to the connection's buffered writer.
func (c *ClientConn) Write(buf []byte) (int, error) {
	return c.bw.Write(buf)
}

// ColorMap returns the color map for the connection.
func (c *ClientConn) ColorMap() ColorMap {
	return c.colorMap
}

// SetColorMap sets the color map for the connection.
func (c *ClientConn) SetColorMap(cm ColorMap) {
	c.colorMap = cm
}

// DesktopName returns the desktop name of the remote session.
func (c *ClientConn) DesktopName() []byte {
	return c.desktopName
}

// PixelFormat returns the pixel format of the connection.
func (c *ClientConn) PixelFormat() PixelFormat {
	return c.pixelFormat
}

// SetDesktopName sets the desktop name.
func (c *ClientConn) SetDesktopName(name []byte) {
	c.desktopName = name
}

// SetPixelFormat sets the pixel format for the connection.
func (c *ClientConn) SetPixelFormat(pf PixelFormat) error {
	c.pixelFormat = pf
	return nil
}

// Encodings returns the list of supported encoding handlers.
func (c *ClientConn) Encodings() []Encoding {
	return c.encodings
}

// Width returns the framebuffer width.
func (c *ClientConn) Width() uint16 {
	return c.fbWidth
}

// Height returns the framebuffer height.
func (c *ClientConn) Height() uint16 {
	return c.fbHeight
}

// Protocol returns the negotiated VNC protocol version.
func (c *ClientConn) Protocol() string {
	return c.protocol
}

// SetWidth sets the framebuffer width.
func (c *ClientConn) SetWidth(width uint16) {
	c.fbWidth = width
}

// SetHeight sets the framebuffer height.
func (c *ClientConn) SetHeight(height uint16) {
	c.fbHeight = height
}

// SecurityHandler returns the security handler for the connection.
func (c *ClientConn) SecurityHandler() SecurityHandler {
	return c.securityHandler
}

// SetSecurityHandler sets the security handler.
func (c *ClientConn) SetSecurityHandler(sechandler SecurityHandler) error {
	c.securityHandler = sechandler
	return nil
}

// ResetAllEncodings resets the internal state of all supported encoding handlers.
func (c *ClientConn) ResetAllEncodings() {
	for _, enc := range c.encodings {
		enc.Reset()
	}
}

// DefaultClientMessageHandler is the default handler for processing server messages
// after the handshake is complete. It starts the main message handling loops.
type DefaultClientMessageHandler struct{}

// Handle starts the message handling loops for the client.
func (*DefaultClientMessageHandler) Handle(c Conn) error {
	logger.Trace("starting DefaultClientMessageHandler")
	clientConn, ok := c.(*ClientConn)
	if !ok {
		return errors.New("handler expected a *ClientConn")
	}
	cfg := clientConn.cfg

	// Create a map of server message types to their handlers for quick lookup.
	serverMessages := make(map[ServerMessageType]ServerMessage)
	for _, m := range cfg.Messages {
		serverMessages[m.Type()] = m
	}

	// Start the goroutines for handling incoming and outgoing messages.
	clientConn.wg.Add(2)
	go clientConn.handleIncomingMessages(serverMessages)
	go clientConn.handleOutgoingMessages()

	// Set the client's supported encodings on the server.
	var encTypes []EncodingType
	for _, enc := range clientConn.Encodings() {
		encTypes = append(encTypes, enc.Type())
	}
	logger.Tracef("setting encodings: %v", encTypes)
	if err := clientConn.SetEncodings(encTypes); err != nil {
		return fmt.Errorf("failed to set encodings: %w", err)
	}

	// Send the initial framebuffer update request.
	req := FramebufferUpdateRequest{
		Inc:    1, // Request an incremental update.
		X:      0,
		Y:      0,
		Width:  clientConn.Width(),
		Height: clientConn.Height(),
	}
	logger.Tracef("sending initial framebuffer update request: %+v", req)
	return req.Write(clientConn)
}

// handleIncomingMessages runs in a dedicated goroutine, reading and processing
// messages from the server.
func (c *ClientConn) handleIncomingMessages(serverMessages map[ServerMessageType]ServerMessage) {
	defer c.wg.Done()
	defer c.Close() // Ensure connection is closed if this loop exits.

	for {
		// Set a read deadline to detect idle or hung connections.
		// The deadline is extended each time a message is successfully read.
		c.c.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Check for quit signal without blocking.
		select {
		case <-c.quit:
			return
		default:
		}

		var msgType ServerMessageType
		if err := binary.Read(c, binary.BigEndian, &msgType); err != nil {
			// A read error, often io.EOF, means the connection is closed.
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				logger.Errorf("error reading message type: %v", err)
			}
			return
		}

		msg, ok := serverMessages[msgType]
		if !ok {
			// Include the remote address for easier debugging of client issues.
			logger.Errorf("unsupported message type %d from server %s", msgType, c.c.RemoteAddr())
			return // Unknown message type is a fatal error.
		}

		if c.Canvas != nil {
			c.Canvas.RemoveCursor()
		}

		parsedMsg, err := msg.Read(c)
		if err != nil {
			logger.Errorf("error reading message body for type %d: %v", msgType, err)
			return
		}

		if c.Canvas != nil {
			c.Canvas.PaintCursor()
		}

		// Send the parsed message to the application logic.
		select {
		case c.cfg.ServerMessageCh <- parsedMsg:
		case <-c.quit:
			return
		}
	}
}

// handleOutgoingMessages runs in a dedicated goroutine, sending messages
// from the client to the server.
func (c *ClientConn) handleOutgoingMessages() {
	defer c.wg.Done()

	for {
		select {
		case msg, ok := <-c.cfg.ClientMessageCh:
			if !ok {
				// Channel closed, which is a signal to shut down.
				return
			}
			if err := msg.Write(c); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					logger.Errorf("error writing message: %v", err)
				}
				c.Close()
				return
			}
			if err := c.Flush(); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					logger.Errorf("error flushing writer: %v", err)
				}
				return
			}
		case <-c.quit:
			return
		}
	}
}
