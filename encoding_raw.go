package avacadovnc

import (
	"fmt"
	"io"
)

// RawEncoding implements the raw encoding, which is the simplest and most
// inefficient encoding. It sends uncompressed pixel data.
type RawEncoding struct{}

// Type returns the encoding type identifier.
func (e *RawEncoding) Type() EncodingType {
	return EncRaw
}

// Read decodes a rectangle of raw, uncompressed pixel data.
func (e *RawEncoding) Read(c Conn, rect *Rectangle) error {
	bytesPerPixel := c.PixelFormat().BPP
	if bytesPerPixel == 0 {
		return fmt.Errorf("raw: bytes per pixel is zero")
	}

	// Calculate the total number of bytes for the rectangle.
	bytesToRead := int(rect.Width) * int(rect.Height) * int(bytesPerPixel)
	if bytesToRead == 0 {
		return nil // Nothing to read.
	}

	pixelData := make([]byte, bytesToRead)
	if _, err := io.ReadFull(c, pixelData); err != nil {
		return fmt.Errorf("raw: failed to read pixel data: %w", err)
	}

	// Draw the decoded bytes to the canvas.
	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil // No canvas to draw on.
	}

	return clientConn.Canvas.DrawBytes(pixelData, rect)
}

// Reset does nothing as this encoding is stateless.
func (e *RawEncoding) Reset() {}
