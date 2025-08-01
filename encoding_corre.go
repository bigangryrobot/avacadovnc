package avacadovnc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// CoRREEncoding implements the CoRRE (Compressed RRE) encoding.
// It is a variation of RRE that uses a palette for colors.
type CoRREEncoding struct{}

// Type returns the encoding type identifier.
func (e *CoRREEncoding) Type() EncodingType {
	return EncCoRRE
}

// Read decodes CoRRE-encoded data.
func (e *CoRREEncoding) Read(c Conn, rect *Rectangle) error {
	var numSubRects uint32
	if err := binary.Read(c, binary.BigEndian, &numSubRects); err != nil {
		return fmt.Errorf("corre: failed to read number of sub-rectangles: %w", err)
	}

	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil // No canvas to draw on, but we must still read the data.
	}

	pf := c.PixelFormat()
	bytesPerPixel := pf.BytesPerPixel()
	if bytesPerPixel == 0 {
		return fmt.Errorf("corre: bytes per pixel is zero")
	}

	// The background color is sent first.
	bgBytes := make([]byte, bytesPerPixel)
	if _, err := io.ReadFull(c, bgBytes); err != nil {
		return fmt.Errorf("corre: failed to read background color: %w", err)
	}

	if clientConn.Canvas != nil {
		clientConn.Canvas.Fill(bgBytes, rect)
	}

	// Read and process each sub-rectangle.
	for i := uint32(0); i < numSubRects; i++ {
		colorBytes := make([]byte, bytesPerPixel)
		if _, err := io.ReadFull(c, colorBytes); err != nil {
			return fmt.Errorf("corre: failed to read sub-rectangle color: %w", err)
		}

		var subRect Rectangle
		if err := binary.Read(c, binary.BigEndian, &subRect); err != nil {
			return fmt.Errorf("corre: failed to read sub-rectangle header: %w", err)
		}

		if clientConn.Canvas != nil {
			// Adjust sub-rectangle position to be relative to the main canvas.
			subRect.X += rect.X
			subRect.Y += rect.Y
			clientConn.Canvas.Fill(colorBytes, &subRect)
		}
	}

	return nil
}

// Reset does nothing as this encoding is stateless.
func (e *CoRREEncoding) Reset() {}
