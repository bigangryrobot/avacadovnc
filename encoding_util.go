package vnc2video

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"io"
)

// ReadPixel reads the raw bytes for a single pixel from the reader based on the
// connection's pixel format and returns them packed into a uint32.
// This function handles different bytes-per-pixel (BPP) values and endianness,
// providing a normalized format for further processing.
func ReadPixel(r io.Reader, pf *PixelFormat) (uint32, error) {
	var px uint32
	order := pixelOrder(pf)

	switch pf.BPP {
	case 8:
		var px8 uint8
		if err := binary.Read(r, order, &px8); err != nil {
			return 0, fmt.Errorf("failed to read 8-bit pixel: %w", err)
		}
		px = uint32(px8)
	case 16:
		var px16 uint16
		if err := binary.Read(r, order, &px16); err != nil {
			return 0, fmt.Errorf("failed to read 16-bit pixel: %w", err)
		}
		px = uint32(px16)
	case 32:
		var px32 uint32
		if err := binary.Read(r, order, &px32); err != nil {
			return 0, fmt.Errorf("failed to read 32-bit pixel: %w", err)
		}
		px = px32
	default:
		return 0, fmt.Errorf("unsupported BPP: %d", pf.BPP)
	}
	return px, nil
}

// PixelToRGBA converts a raw pixel value (packed in a uint32) to an RGBA color,
// according to the given pixel format and color map. This is the core of color
// translation in the VNC client.
func PixelToRGBA(pixel uint32, pf *PixelFormat, cm *ColorMap) color.RGBA {
	if pf.TrueColor == 0 {
		// Paletted color. The pixel value is an index into the color map.
		if cm != nil && pixel < uint32(len(cm)) {
			return cm[pixel]
		}
		// Fallback if color map is missing or index is out of bounds.
		// This shouldn't happen in a valid VNC session.
		return color.RGBA{R: 0, G: 0, B: 0, A: 255}
	}

	// True color. Extract R, G, B components using bit shifts and masks.
	red := (pixel >> pf.RedShift) & uint32(pf.RedMax)
	green := (pixel >> pf.GreenShift) & uint32(pf.GreenMax)
	blue := (pixel >> pf.BlueShift) & uint32(pf.BlueMax)

	// Scale the components to the full 0-255 range.
	// This is crucial for correctly displaying colors with depths less than 24-bit
	// (e.g., 16-bit color, where RedMax might be 31).
	r := uint8((float64(red) * 255.0) / float64(pf.RedMax))
	g := uint8((float64(green) * 255.0) / float64(pf.GreenMax))
	b := uint8((float64(blue) * 255.0) / float64(pf.BlueMax))

	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// pixelOrder is a helper function to determine the byte order from a PixelFormat.
func pixelOrder(pf *PixelFormat) binary.ByteOrder {
	if pf.BigEndian != 0 {
		return binary.BigEndian
	}
	return binary.LittleEndian
}
