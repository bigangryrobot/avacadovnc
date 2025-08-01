package avacadovnc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// RREEncoding implements the RRE (Rise-and-Run-length Encoding), which is
// efficient for encoding large areas of a single color.
type RREEncoding struct{}

// Type returns the encoding type identifier.
func (e *RREEncoding) Type() EncodingType {
	return EncRRE
}

// Read decodes RRE-encoded data.
func (e *RREEncoding) Read(c Conn, rect *Rectangle) error {
	var numSubRects uint32
	if err := binary.Read(c, binary.BigEndian, &numSubRects); err != nil {
		return fmt.Errorf("rre: failed to read number of sub-rectangles: %w", err)
	}

	clientConn, ok := c.(*ClientConn)
	if !ok {
		return fmt.Errorf("rre: connection is not a client connection")
	}

	pf := c.PixelFormat()
	bytesPerPixel := pf.BPP
	if bytesPerPixel == 0 {
		return fmt.Errorf("rre: bytes per pixel is zero")
	}

	// The first color read is the background color for the entire rectangle.
	bgColor := make([]byte, bytesPerPixel)
	if _, err := io.ReadFull(c, bgColor); err != nil {
		return fmt.Errorf("rre: failed to read background color: %w", err)
	}

	if clientConn.Canvas != nil {
		clientConn.Canvas.Fill(bgColor, rect)
	}

	// Read and process each sub-rectangle.
	for i := uint32(0); i < numSubRects; i++ {
		subRectColor := make([]byte, bytesPerPixel)
		if _, err := io.ReadFull(c, subRectColor); err != nil {
			return fmt.Errorf("rre: failed to read sub-rectangle color: %w", err)
		}

		var subRect Rectangle
		if err := binary.Read(c, binary.BigEndian, &subRect); err != nil {
			return fmt.Errorf("rre: failed to read sub-rectangle header: %w", err)
		}

		if clientConn.Canvas != nil {
			// Adjust sub-rectangle position to be relative to the main canvas.
			subRect.X += rect.X
			subRect.Y += rect.Y
			clientConn.Canvas.Fill(subRectColor, &subRect)
		}
	}

	return nil
}

// Reset does nothing as this encoding is stateless.
func (e *RREEncoding) Reset() {}
