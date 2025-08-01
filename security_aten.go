package avacadovnc

import (
	"encoding/binary"
	"errors"
)

// SecurityAtenHermon implements a vendor-specific security type used by
// some Aten KVM devices.
type SecurityAtenHermon struct {
	// The SubType field was removed during refactoring as it was not used.
}

// Type returns the security type identifier.
func (s *SecurityAtenHermon) Type() SecurityType {
	return SecTypeAtenHermon
}

// Authenticate performs the security handshake. For this specific type,
// it appears to be a no-op on the client side other than checking the result.
func (s *SecurityAtenHermon) Authenticate(c Conn) error {
	// This security handler is client-side only in this implementation.
	if _, ok := c.Config().(*ClientConfig); !ok {
		return errors.New("aten-hermon: server-side authentication not implemented")
	}

	var securityResult uint32
	if err := binary.Read(c, binary.BigEndian, &securityResult); err != nil {
		return err
	}
	if securityResult != 0 {
		return errors.New("aten-hermon: authentication failed")
	}
	return nil
}
