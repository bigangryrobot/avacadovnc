package avacadovnc

import (
	"encoding/binary"
	"errors"
)

// SecurityNone implements the "None" security type (type 1), which involves
// no authentication.
type SecurityNone struct{}

// Type returns the security type identifier.
func (s *SecurityNone) Type() SecurityType {
	return SecTypeNone
}

// Authenticate performs the security handshake for the "None" type.
// For the client, this means reading the security result from the server.
// For the server, it means sending a success result to the client.
func (s *SecurityNone) Authenticate(c Conn) error {
	// The logic differs slightly if this is a client or server connection.
	// A simple way to check is if the config is a ClientConfig.
	if _, ok := c.Config().(*ClientConfig); ok {
		// Client-side implementation
		var securityResult uint32
		if err := binary.Read(c, binary.BigEndian, &securityResult); err != nil {
			return err
		}
		if securityResult != 0 {
			return errors.New("security-none: authentication failed")
		}
		return nil
	}

	// Server-side implementation
	if err := binary.Write(c, binary.BigEndian, uint32(0)); err != nil {
		return err
	}
	return c.Flush()
}
