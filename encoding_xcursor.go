package vnc2video

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
)

// XCursorEncoding implements the XCursor pseudo-encoding, which is an extended
// version of the standard cursor encoding.
type XCursorEncoding struct{}

// Type returns the encoding type identifier.
func (e *XCursorEncoding) Type() EncodingType {
	return EncXCursor
}

// Read decodes the XCursor data from the connection.
func (e *XCursorEncoding) Read(c Conn, rect *Rectangle) error {
	var numColors uint16
	if err := binary.Read(c, binary.BigEndian, &numColors); err != nil {
		return fmt.Errorf("xcursor: failed to read number of colors: %w", err)
	}

	// Read the palette.
	palette := make([]color.RGBA, numColors)
	for i := 0; i < int(numColors); i++ {
		var r, g, b uint16
		if err := binary.Read(c, binary.BigEndian, &r); err != nil {
			return err
		}
		if err := binary.Read(c, binary.BigEndian, &g); err != nil {
			return err
		}
		if err := binary.Read(c, binary.BigEndian, &b); err != nil {
			return err
		}
		palette[i] = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
	}

	// Read packed pixel data and bitmask.
	numPixels := int(rect.Width * rect.Height)
	packedPixels := make([]byte, numPixels)
	if _, err := io.ReadFull(c, packedPixels); err != nil {
		return fmt.Errorf("xcursor: failed to read packed pixels: %w", err)
	}

	bitmask := make([]byte, numPixels)
	if _, err := io.ReadFull(c, bitmask); err != nil {
		return fmt.Errorf("xcursor: failed to read bitmask: %w", err)
	}

	// A client with a UI would use this data to render the cursor.
	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil // No canvas to draw on.
	}

	cursorImg := image.NewRGBA(image.Rect(0, 0, int(rect.Width), int(rect.Height)))
	cursorMask := image.NewAlpha(image.Rect(0, 0, int(rect.Width), int(rect.Height)))

	for i := 0; i < numPixels; i++ {
		x, y := i%int(rect.Width), i/int(rect.Height)
		if int(packedPixels[i]) < len(palette) {
			cursorImg.Set(x, y, palette[packedPixels[i]])
		}
		if int(bitmask[i]) < len(palette) {
			// The alpha value is determined by the corresponding pixel in the bitmask data
			alpha := palette[bitmask[i]].R // Using R as grayscale for alpha
			cursorMask.SetAlpha(x, y, color.Alpha{A: alpha})
		}
	}

	clientConn.Canvas.SetCursor(cursorImg, cursorMask, int(rect.X), int(rect.Y))
	return nil
}

// Reset does nothing as this encoding is stateless.
func (e *XCursorEncoding) Reset() {}
