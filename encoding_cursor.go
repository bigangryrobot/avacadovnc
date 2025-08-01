package avacadovnc

import (
	"fmt"
	"image"
	"image/color" // Added missing import
	"io"
)

// CursorEncoding implements the Cursor pseudo-encoding, which is used to
// update the client's mouse cursor image.
type CursorEncoding struct{}

// Type returns the encoding type identifier.
func (e *CursorEncoding) Type() EncodingType {
	return EncCursor
}

// Read decodes the cursor data from the connection.
func (e *CursorEncoding) Read(c Conn, rect *Rectangle) error {
	bytesPerPixel := c.PixelFormat().BPP
	if bytesPerPixel == 0 {
		return fmt.Errorf("cursor encoding: bytes per pixel is zero")
	}

	// The cursor data is a bitmap followed by a bitmask.
	// Each is Width * Height pixels.
	numPixels := int(rect.Width * rect.Height)
	bitmapBytes := make([]byte, numPixels*int(bytesPerPixel))
	if _, err := io.ReadFull(c, bitmapBytes); err != nil {
		return fmt.Errorf("cursor encoding: failed to read bitmap: %w", err)
	}

	bitmaskBytes := make([]byte, (int(rect.Width)+7)/8*int(rect.Height))
	if _, err := io.ReadFull(c, bitmaskBytes); err != nil {
		return fmt.Errorf("cursor encoding: failed to read bitmask: %w", err)
	}

	// A client with a UI would use this data to render the cursor.
	// For example, by creating an image.RGBA and an image.Alpha mask.
	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil // No canvas to draw on.
	}

	cursorImg := image.NewRGBA(image.Rect(0, 0, int(rect.Width), int(rect.Height)))
	cursorMask := image.NewAlpha(image.Rect(0, 0, int(rect.Width), int(rect.Height)))

	// Populate the cursor image and mask.
	// This is a simplified conversion and may need adjustment based on pixel format.
	for i := 0; i < numPixels; i++ {
		x, y := i%int(rect.Width), i/int(rect.Width)
		// This assumes 32bpp RGBA format for simplicity.
		if bytesPerPixel == 4 {
			cursorImg.SetRGBA(x, y, color.RGBA{
				R: bitmapBytes[i*4+2], // Assuming BGRX order from some servers
				G: bitmapBytes[i*4+1],
				B: bitmapBytes[i*4+0],
				A: 255,
			})
		}
		// Set alpha mask
		if (bitmaskBytes[y*((int(rect.Width)+7)/8)+x/8]>>(7-x%8))&1 != 0 {
			cursorMask.SetAlpha(x, y, color.Alpha{A: 255})
		}
	}

	clientConn.Canvas.SetCursor(cursorImg, cursorMask, int(rect.X), int(rect.Y))
	return nil
}

// Reset does nothing as this encoding is stateless.
func (e *CursorEncoding) Reset() {}
