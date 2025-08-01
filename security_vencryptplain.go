package avacadovnc

import (
	"encoding/binary"
	"errors"
)

// SecurityVeNCryptPlain implements the "Plain" sub-type of the VeNCrypt security
// scheme, which is used for unencrypted sessions within the VeNCrypt framework.
type SecurityVeNCryptPlain struct {
	// The SubType field was removed during refactoring.
}

// Type returns the security type identifier.
func (s *SecurityVeNCryptPlain) Type() SecurityType {
	return SecTypeVeNCrypt
}

// Authenticate performs the security handshake.
func (s *SecurityVeNCryptPlain) Authenticate(c Conn) error {
	// This security handler is client-side only in this implementation.
	if _, ok := c.Config().(*ClientConfig); !ok {
		return errors.New("vencrypt-plain: server-side authentication not implemented")
	}

	var securityResult uint32
	if err := binary.Read(c, binary.BigEndian, &securityResult); err != nil {
		return err
	}
	if securityResult != 0 {
		return errors.New("vencrypt-plain: authentication failed")
	}
	return nil
}
