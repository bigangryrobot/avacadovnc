package vnc2video

import (
	"encoding/binary"
	"fmt"
	"image"

)

// CopyRectEncoding implements the CopyRect encoding, which is used to copy a
// rectangular area from one part of the framebuffer to another.
type CopyRectEncoding struct{}

// Type returns the encoding type identifier.
func (e *CopyRectEncoding) Type() EncodingType {
	return EncCopyRect
}

// Read decodes the CopyRect data, which consists of the source X and Y coordinates.
func (e *CopyRectEncoding) Read(c Conn, rect *Rectangle) error {
	var srcX, srcY uint16
	if err := binary.Read(c, binary.BigEndian, &srcX); err != nil {
		return fmt.Errorf("copyrect: failed to read source X: %w", err)
	}
	if err := binary.Read(c, binary.BigEndian, &srcY); err != nil {
		return fmt.Errorf("copyrect: failed to read source Y: %w", err)
	}

	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil // No canvas to draw on.
	}

	// Perform the copy operation on the canvas.
	srcPoint := image.Point{int(srcX), int(srcY)}
	dstPoint := image.Point{int(rect.X), int(rect.Y)}
	size := image.Point{int(rect.Width), int(rect.Height)}

	return clientConn.Canvas.Copy(srcPoint, dstPoint, size)
}

// Reset conforms to the Encoding interface.
// This was changed from `Reset() error` to `Reset()` to match the interface.
func (e *CopyRectEncoding) Reset() {
	// No state to reset in this implementation.
}
