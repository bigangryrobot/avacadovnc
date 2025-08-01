package vnc2video

import (
	"fmt"
)

// PointerPosEncoding implements the PointerPos pseudo-encoding.
// This is not a true encoding but a message from the server to update the
// client-side position of the mouse cursor.
type PointerPosEncoding struct{}

// Type returns the encoding type identifier.
func (e *PointerPosEncoding) Type() EncodingType {
	return EncPointerPos
}

// Read handles the cursor position update. The new coordinates are in the
// rectangle header's X and Y fields.
func (e *PointerPosEncoding) Read(c Conn, rect *Rectangle) error {
	// The new cursor position is sent in the X and Y fields of the rectangle header.
	newX := rect.X
	newY := rect.Y

	// Get the client connection to access its canvas.
	clientConn, ok := c.(*ClientConn)
	if !ok {
		// This encoding is only valid for clients.
		return fmt.Errorf("pointer-pos: connection is not a client connection")
	}

	if clientConn.Canvas == nil {
		// No canvas to update, so we can ignore this message.
		return nil
	}

	// Update the cursor's location on the canvas.
	clientConn.Canvas.MoveCursor(int(newX), int(newY))

	return nil
}

// Reset does nothing as this encoding is stateless.
func (e *PointerPosEncoding) Reset() {}
