package avacadovnc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// AtenHermonEncoding implements a vendor-specific encoding used by some Aten KVMs.
// It acts as a container for various sub-encodings.
type AtenHermonEncoding struct {
	subEncodings []Encoding
}

// Type returns the encoding type identifier.
func (e *AtenHermonEncoding) Type() EncodingType {
	return EncAtenHermon
}

// Read decodes the AtenHermon data, which involves reading a sub-encoding
// type and delegating to the appropriate handler.
func (e *AtenHermonEncoding) Read(c Conn, rect *Rectangle) error {
	var subEncodingType uint8
	if err := binary.Read(c, binary.BigEndian, &subEncodingType); err != nil {
		return fmt.Errorf("aten-hermon: failed to read sub-encoding type: %w", err)
	}

	switch subEncodingType {
	case 129: // AtenHermonSubrect
		encSR := &AtenHermonSubrect{}
		e.subEncodings = append(e.subEncodings, encSR)
		// The problematic line `rect.EncType = ...` has been removed here,
		// as the Rectangle struct no longer holds the encoding type.
		return encSR.Read(c, rect)
	default:
		return fmt.Errorf("aten-hermon: unsupported sub-encoding: %d", subEncodingType)
	}
}

// Reset resets the state of all sub-encodings.
func (e *AtenHermonEncoding) Reset() {
	for _, enc := range e.subEncodings {
		enc.Reset()
	}
}

// AtenHermonSubrect is a sub-encoding within the AtenHermon scheme.
type AtenHermonSubrect struct{}

// Type returns the encoding type identifier.
func (s *AtenHermonSubrect) Type() EncodingType {
	// This is a sub-encoding and doesn't have a top-level type.
	// We return the parent's type as a placeholder.
	return EncAtenHermon
}

// Read decodes the sub-rectangle data.
func (s *AtenHermonSubrect) Read(c Conn, rect *Rectangle) error {
	// The implementation for this sub-encoding appears to be a no-op,
	// simply consuming a fixed number of bytes.
	var unknown [8]byte
	if _, err := io.ReadFull(c, unknown[:]); err != nil {
		return fmt.Errorf("aten-hermon-subrect: failed to read data: %w", err)
	}
	return nil
}

// Reset conforms to the Encoding interface.
func (s *AtenHermonSubrect) Reset() {
	// No state to reset in this implementation.
}
