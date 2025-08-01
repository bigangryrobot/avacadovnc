package avacadovnc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// DesktopNameEncoding implements the DesktopName pseudo-encoding, which is used
// by the server to update the client with the session's name.
type DesktopNameEncoding struct{}

// Type returns the encoding type identifier.
func (e *DesktopNameEncoding) Type() EncodingType {
	return EncDesktopName
}

// Read decodes the desktop name data.
func (e *DesktopNameEncoding) Read(c Conn, rect *Rectangle) error {
	var nameLength uint32
	if err := binary.Read(c, binary.BigEndian, &nameLength); err != nil {
		return fmt.Errorf("desktop-name: failed to read name length: %w", err)
	}

	name := make([]byte, nameLength)
	if _, err := io.ReadFull(c, name); err != nil {
		return fmt.Errorf("desktop-name: failed to read name: %w", err)
	}

	c.SetDesktopName(name)
	return nil
}

// Reset does nothing as this encoding is stateless.
func (e *DesktopNameEncoding) Reset() {}
