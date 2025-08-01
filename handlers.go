package avacadovnc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// ProtocolVersion is the VNC protocol version this library supports.
	ProtocolVersion = "RFB 003.008\n"
)

// --- Client Handlers ---

// DefaultClientVersionHandler handles the protocol version negotiation for the client.
type DefaultClientVersionHandler struct{}

// Handle reads the server's protocol version and sends the client's version.
func (h *DefaultClientVersionHandler) Handle(c Conn) error {
	var serverVersion [12]byte
	if _, err := io.ReadFull(c, serverVersion[:]); err != nil {
		return fmt.Errorf("failed to read server version: %w", err)
	}

	// For now, we don't do anything with the server's version, but a more
	// robust client might check it for compatibility.
	c.SetProtoVersion(string(serverVersion[:]))

	if _, err := c.Write([]byte(ProtocolVersion)); err != nil {
		return fmt.Errorf("failed to write client version: %w", err)
	}
	return c.Flush()
}

// DefaultClientSecurityHandler handles the security negotiation for the client.
type DefaultClientSecurityHandler struct{}

// Handle negotiates a security type with the server and performs authentication.
func (h *DefaultClientSecurityHandler) Handle(c Conn) error {
	var numSecTypes uint8
	if err := binary.Read(c, binary.BigEndian, &numSecTypes); err != nil {
		return fmt.Errorf("failed to read number of security types: %w", err)
	}

	if numSecTypes == 0 {
		// If the server sends 0 security types, it's followed by a reason string.
		var reasonLen uint32
		if err := binary.Read(c, binary.BigEndian, &reasonLen); err != nil {
			return fmt.Errorf("failed to read security failure reason length: %w", err)
		}
		reason := make([]byte, reasonLen)
		if _, err := io.ReadFull(c, reason); err != nil {
			return fmt.Errorf("failed to read security failure reason: %w", err)
		}
		return fmt.Errorf("server reported security failure: %s", reason)
	}

	// Read the raw security types into a byte slice.
	serverSecTypesBytes := make([]byte, numSecTypes)
	if _, err := io.ReadFull(c, serverSecTypesBytes); err != nil {
		return fmt.Errorf("failed to read server security types: %w", err)
	}

	// Convert the byte slice to a slice of SecurityType.
	serverSecTypes := make([]SecurityType, numSecTypes)
	for i, b := range serverSecTypesBytes {
		serverSecTypes[i] = SecurityType(b)
	}

	cfg, ok := c.Config().(*ClientConfig)
	if !ok {
		return errors.New("invalid connection config type for client")
	}

	// Find the first security handler supported by both client and server.
	for _, clientHandler := range cfg.SecurityHandlers {
		for _, serverSecType := range serverSecTypes {
			if clientHandler.Type() == serverSecType {
				// We found a match. Send our choice to the server.
				if _, err := c.Write([]byte{byte(clientHandler.Type())}); err != nil {
					return fmt.Errorf("failed to write security type: %w", err)
				}
				if err := c.Flush(); err != nil {
					return err
				}
				// Perform authentication.
				c.SetSecurityHandler(clientHandler)
				return clientHandler.Authenticate(c)
			}
		}
	}

	return errors.New("no supported security types found")
}

// DefaultClientClientInitHandler sends the ClientInit message.
type DefaultClientClientInitHandler struct{}

// Handle sends the "shared" flag to the server.
func (h *DefaultClientClientInitHandler) Handle(c Conn) error {
	cfg, ok := c.Config().(*ClientConfig)
	if !ok {
		return errors.New("invalid connection config type for client")
	}

	var sharedFlag uint8
	if !cfg.Exclusive {
		sharedFlag = 1
	}

	if _, err := c.Write([]byte{sharedFlag}); err != nil {
		return fmt.Errorf("failed to write shared flag: %w", err)
	}
	return c.Flush()
}

// DefaultClientServerInitHandler reads the ServerInit message.
type DefaultClientServerInitHandler struct{}

// Handle reads the server's framebuffer dimensions, pixel format, and desktop name.
func (h *DefaultClientServerInitHandler) Handle(c Conn) error {
	var width, height uint16
	if err := binary.Read(c, binary.BigEndian, &width); err != nil {
		return err
	}
	if err := binary.Read(c, binary.BigEndian, &height); err != nil {
		return err
	}
	c.SetWidth(width)
	c.SetHeight(height)

	var pf PixelFormat
	if err := binary.Read(c, binary.BigEndian, &pf); err != nil {
		return err
	}
	c.SetPixelFormat(pf)

	var nameLength uint32
	if err := binary.Read(c, binary.BigEndian, &nameLength); err != nil {
		return err
	}
	name := make([]byte, nameLength)
	if _, err := io.ReadFull(c, name); err != nil {
		return err
	}
	c.SetDesktopName(name)

	return nil
}

// --- Server Handlers ---

// DefaultServerVersionHandler handles protocol version negotiation for the server.
type DefaultServerVersionHandler struct{}

// Handle sends the server's version and reads the client's.
func (h *DefaultServerVersionHandler) Handle(c Conn) error {
	if _, err := c.Write([]byte(ProtocolVersion)); err != nil {
		return fmt.Errorf("failed to write server version: %w", err)
	}
	if err := c.Flush(); err != nil {
		return err
	}

	var clientVersion [12]byte
	if _, err := io.ReadFull(c, clientVersion[:]); err != nil {
		return fmt.Errorf("failed to read client version: %w", err)
	}

	// A real server might validate the client version.
	if !bytes.HasPrefix(clientVersion[:], []byte("RFB")) {
		return fmt.Errorf("invalid client version signature: %s", clientVersion)
	}

	return nil
}

// DefaultServerSecurityHandler handles security negotiation for the server.
type DefaultServerSecurityHandler struct{}

// Handle sends supported security types and authenticates the client.
func (h *DefaultServerSecurityHandler) Handle(c Conn) error {
	cfg, ok := c.Config().(*ServerConfig)
	if !ok {
		return errors.New("invalid connection config type for server")
	}

	secTypes := make([]byte, len(cfg.SecurityHandlers))
	for i, handler := range cfg.SecurityHandlers {
		secTypes[i] = byte(handler.Type())
	}

	if _, err := c.Write([]byte{uint8(len(secTypes))}); err != nil {
		return fmt.Errorf("failed to write number of security types: %w", err)
	}
	if _, err := c.Write(secTypes); err != nil {
		return fmt.Errorf("failed to write security types: %w", err)
	}
	if err := c.Flush(); err != nil {
		return err
	}

	var clientChoice uint8
	if err := binary.Read(c, binary.BigEndian, &clientChoice); err != nil {
		return fmt.Errorf("failed to read client security choice: %w", err)
	}

	for _, handler := range cfg.SecurityHandlers {
		if handler.Type() == SecurityType(clientChoice) {
			c.SetSecurityHandler(handler)
			return handler.Authenticate(c)
		}
	}

	return fmt.Errorf("client chose an unsupported security type: %d", clientChoice)
}

// DefaultServerClientInitHandler reads the ClientInit message on the server.
type DefaultServerClientInitHandler struct{}

// Handle reads the client's "shared" flag.
func (h *DefaultServerClientInitHandler) Handle(c Conn) error {
	var sharedFlag [1]byte
	if _, err := io.ReadFull(c, sharedFlag[:]); err != nil {
		return fmt.Errorf("failed to read client init (shared flag): %w", err)
	}
	// A real server would use this flag to manage session sharing.
	return nil
}

// DefaultServerServerInitHandler sends the ServerInit message.
type DefaultServerServerInitHandler struct{}

// Handle sends the server's framebuffer info to the client.
func (h *DefaultServerServerInitHandler) Handle(c Conn) error {
	cfg, ok := c.Config().(*ServerConfig)
	if !ok {
		return errors.New("invalid connection config type for server")
	}

	if err := binary.Write(c, binary.BigEndian, cfg.Width); err != nil {
		return err
	}
	if err := binary.Write(c, binary.BigEndian, cfg.Height); err != nil {
		return err
	}
	if err := binary.Write(c, binary.BigEndian, cfg.PixelFormat); err != nil {
		return err
	}

	name := []byte(cfg.DesktopName)
	if err := binary.Write(c, binary.BigEndian, uint32(len(name))); err != nil {
		return err
	}
	if _, err := c.Write(name); err != nil {
		return err
	}

	return c.Flush()
}
