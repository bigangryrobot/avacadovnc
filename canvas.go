package avacadovnc

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"sync"
)

// VncCanvas represents the client's view of the remote framebuffer.
// It provides a drawable surface (an image.RGBA) and methods to manipulate it
// based on messages received from the server. It is safe for concurrent use.
type VncCanvas struct {
	mu          sync.RWMutex // Use RWMutex for more granular locking
	img         *image.RGBA  // The main framebuffer image
	cursorImg   *image.RGBA  // The cursor image
	cursorMask  *image.Alpha // The cursor bitmask for transparency
	cursorX     int          // Cursor X position
	cursorY     int          // Cursor Y position
	cursorHotX  int          // Cursor hotspot X
	cursorHotY  int          // Cursor hotspot Y
	cursorShown bool
}

// NewVncCanvas creates a new canvas with the specified dimensions.
func NewVncCanvas(width, height int, pf PixelFormat) *VncCanvas {
	return &VncCanvas{
		img: image.NewRGBA(image.Rect(0, 0, width, height)),
	}
}

// Width returns the width of the canvas.
func (c *VncCanvas) Width() int {
	c.mu.RLock() // Use a read lock for read-only operations
	defer c.mu.RUnlock()
	return c.img.Bounds().Dx()
}

// Height returns the height of the canvas.
func (c *VncCanvas) Height() int {
	c.mu.RLock() // Use a read lock for read-only operations
	defer c.mu.RUnlock()
	return c.img.Bounds().Dy()
}

// Image returns a copy of the current framebuffer image.
// This is safe to use concurrently while the canvas is being updated.
func (c *VncCanvas) Image() *image.RGBA {
	c.mu.RLock() // Use a read lock
	defer c.mu.RUnlock()
	// Return a copy to prevent race conditions if the caller modifies the image.
	clone := *c.img
	// Copy the pixel data as well.
	clone.Pix = make([]byte, len(c.img.Pix))
	copy(clone.Pix, c.img.Pix)
	return &clone
}

// Draw updates a rectangular area of the canvas with the given image.
// This is a general-purpose drawing function. The signature is changed from
// draw.Image to image.Image to resolve the compiler error, as the source
// image for a draw operation only needs to be readable.
func (c *VncCanvas) Draw(img image.Image, rect *Rectangle) {
	c.mu.Lock() // Use a full write lock for modifications
	defer c.mu.Unlock()
	r := image.Rect(int(rect.X), int(rect.Y), int(rect.X+rect.Width), int(rect.Y+rect.Height))
	draw.Draw(c.img, r, img, image.Point{0, 0}, draw.Src)
}

// DrawBytes updates a rectangular area with raw pixel data.
// The format of the pixel data is assumed to match the canvas's 32-bit RGBA format.
func (c *VncCanvas) DrawBytes(pixelData []byte, rect *Rectangle) error {
	c.mu.Lock() // Use a full write lock for modifications
	defer c.mu.Unlock()
	return c.drawBytes(pixelData, rect)
}

// drawBytes is the internal, non-locking version of DrawBytes.
func (c *VncCanvas) drawBytes(pixelData []byte, rect *Rectangle) error {
	img := &image.RGBA{
		Pix:    pixelData,
		Stride: int(rect.Width) * 4, // Assuming 32bpp RGBA
		Rect:   image.Rect(0, 0, int(rect.Width), int(rect.Height)),
	}
	r := image.Rect(int(rect.X), int(rect.Y), int(rect.X+rect.Width), int(rect.Y+rect.Height))
	draw.Draw(c.img, r, img, image.Point{0, 0}, draw.Src)
	return nil
}

// DrawPalette updates a rectangular area with indexed palette data.
func (c *VncCanvas) DrawPalette(indexedData, paletteData []byte, bitsPerIndex int, rect *Rectangle) error {
	c.mu.Lock() // Use a full write lock for modifications
	defer c.mu.Unlock()

	// This is a complex operation that requires converting indexed data to RGBA.
	if bitsPerIndex != 1 && bitsPerIndex != 8 {
		return errors.New("unsupported bitsPerIndex for palette")
	}

	rgbaData := make([]byte, int(rect.Width)*int(rect.Height)*4)
	bytesPerPixel := len(paletteData) / 256 // Assuming full 256-color palette data format

	if bitsPerIndex == 8 {
		for i, index := range indexedData {
			offset := int(index) * bytesPerPixel
			rgbaData[i*4] = paletteData[offset]
			rgbaData[i*4+1] = paletteData[offset+1]
			rgbaData[i*4+2] = paletteData[offset+2]
			rgbaData[i*4+3] = 255
		}
	} else { // bitsPerIndex == 1
		for i := 0; i < len(rgbaData)/4; i++ {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8)
			paletteIndex := (indexedData[byteIndex] >> bitIndex) & 1
			offset := int(paletteIndex) * bytesPerPixel
			rgbaData[i*4] = paletteData[offset]
			rgbaData[i*4+1] = paletteData[offset+1]
			rgbaData[i*4+2] = paletteData[offset+2]
			rgbaData[i*4+3] = 255
		}
	}

	return c.drawBytes(rgbaData, rect)
}

// Fill fills a rectangular area of the canvas with a single color.
func (c *VncCanvas) Fill(colorBytes []byte, rect *Rectangle) error {
	c.mu.Lock() // Use a full write lock for modifications
	defer c.mu.Unlock()
	col := color.RGBA{A: 255}
	if len(colorBytes) >= 3 {
		// Assuming BGR order, common in VNC.
		col.R = colorBytes[2]
		col.G = colorBytes[1]
		col.B = colorBytes[0]
	}
	if len(colorBytes) == 4 {
		col.A = colorBytes[3]
	}
	return c.fill(col, rect)
}

// fill is the internal, non-locking version of Fill.
func (c *VncCanvas) fill(col color.Color, rect *Rectangle) error {
	r := image.Rect(int(rect.X), int(rect.Y), int(rect.X+rect.Width), int(rect.Y+rect.Height))
	draw.Draw(c.img, r, &image.Uniform{C: col}, image.Point{}, draw.Src)
	return nil
}

// Copy performs a screen-to-screen copy. This is used by the CopyRect encoding
// and maps directly to a Guacamole `copy` instruction.
func (c *VncCanvas) Copy(src, dst, size image.Point) error {
	c.mu.Lock() // Use a full write lock for modifications
	defer c.mu.Unlock()

	dstRect := image.Rect(dst.X, dst.Y, dst.X+size.X, dst.Y+size.Y)

	draw.Draw(c.img, dstRect, c.img, src, draw.Src)
	return nil
}

// SetCursor sets the cursor image and hotspot.
func (c *VncCanvas) SetCursor(cursorImg *image.RGBA, cursorMask *image.Alpha, hotX, hotY int) {
	c.mu.Lock() // Use a full write lock for modifications
	defer c.mu.Unlock()
	c.cursorImg = cursorImg
	c.cursorMask = cursorMask
	c.cursorHotX = hotX
	c.cursorHotY = hotY
}

// MoveCursor moves the cursor to a new position.
func (c *VncCanvas) MoveCursor(x, y int) {
	c.mu.Lock() // Use a full write lock for modifications
	defer c.mu.Unlock()
	c.cursorX = x
	c.cursorY = y
}

// PaintCursor draws the cursor onto the framebuffer image.
func (c *VncCanvas) PaintCursor() {
	c.mu.Lock() // Use a full write lock for modifications
	defer c.mu.Unlock()
	if c.cursorImg == nil || c.cursorShown {
		return
	}
	r := c.cursorImg.Bounds().Add(image.Point{c.cursorX - c.cursorHotX, c.cursorY - c.cursorHotY})
	draw.DrawMask(c.img, r, c.cursorImg, image.Point{}, c.cursorMask, image.Point{}, draw.Over)
	c.cursorShown = true
}

// RemoveCursor is a placeholder. A real implementation needs to restore the
// background under the cursor, which requires saving it before painting.
func (c *VncCanvas) RemoveCursor() {
	c.mu.Lock() // Use a full write lock for modifications
	defer c.mu.Unlock()
	// To properly remove a cursor, you would need to have saved the pixels
	// that were underneath it before it was painted. Then you would restore
	// them here. For now, we rely on the server to send updates for the region.
	c.cursorShown = false
}
