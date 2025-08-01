package avacadovnc

import (
	"crypto/des"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// SecurityVNC implements the VNC Challenge-Handshake Authentication protocol.
type SecurityVNC struct {
	Password []byte
}

// Type returns the security type identifier.
func (s *SecurityVNC) Type() SecurityType {
	return SecTypeVNCAuth
}

// Authenticate performs the VNC Auth handshake.
func (s *SecurityVNC) Authenticate(c Conn) error {
	// The logic differs for client and server.
	if _, ok := c.Config().(*ClientConfig); ok {
		return s.authenticateClient(c)
	}
	return s.authenticateServer(c)
}

func (s *SecurityVNC) authenticateClient(c Conn) error {
	var challenge [16]byte
	if _, err := io.ReadFull(c, challenge[:]); err != nil {
		return fmt.Errorf("vnc-auth: failed to read challenge: %w", err)
	}

	// Key is the user password, padded with nulls to 8 bytes.
	key := make([]byte, 8)
	copy(key, s.Password)

	cipher, err := des.NewCipher(key)
	if err != nil {
		return fmt.Errorf("vnc-auth: failed to create des cipher: %w", err)
	}

	response := make([]byte, 16)
	cipher.Encrypt(response[0:8], challenge[0:8])
	cipher.Encrypt(response[8:16], challenge[8:16])

	if _, err := c.Write(response); err != nil {
		return fmt.Errorf("vnc-auth: failed to write response: %w", err)
	}
	if err := c.Flush(); err != nil {
		return err
	}

	var securityResult uint32
	if err := binary.Read(c, binary.BigEndian, &securityResult); err != nil {
		return err
	}
	if securityResult != 0 {
		return errors.New("vnc-auth: authentication failed")
	}
	return nil
}

func (s *SecurityVNC) authenticateServer(c Conn) error {
	// Server-side VNC auth is not implemented in this library.
	return errors.New("server-side VNC auth is not supported")
}
