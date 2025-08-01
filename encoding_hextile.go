package avacadovnc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// HextileEncoding implements the Hextile encoding, which divides the screen
// into 16x16 tiles to efficiently encode areas with few colors.
type HextileEncoding struct{}

// Type returns the encoding type identifier.
func (e *HextileEncoding) Type() EncodingType {
	return EncHextile
}

// Read decodes Hextile-encoded data.
func (e *HextileEncoding) Read(c Conn, rect *Rectangle) error {
	clientConn, ok := c.(*ClientConn)
	if !ok {
		return fmt.Errorf("hextile: connection is not a client connection")
	}

	bytesPerPixel := c.PixelFormat().BPP
	if bytesPerPixel == 0 {
		return fmt.Errorf("hextile: bytes per pixel is zero")
	}

	var bgColor, fgColor []byte

	for y := rect.Y; y < rect.Y+rect.Height; y += 16 {
		for x := rect.X; x < rect.X+rect.Width; x += 16 {
			tileX := x
			tileY := y
			tileW := uint16(16)
			tileH := uint16(16)

			// Adjust tile dimensions for the last tile in a row/column.
			if tileX+tileW > rect.X+rect.Width {
				tileW = rect.X + rect.Width - tileX
			}
			if tileY+tileH > rect.Y+rect.Height {
				tileH = rect.Y + rect.Height - tileY
			}

			var subEncoding uint8
			if err := binary.Read(c, binary.BigEndian, &subEncoding); err != nil {
				return fmt.Errorf("hextile: failed to read sub-encoding mask: %w", err)
			}

			if subEncoding&1 != 0 { // Raw sub-encoding
				rawRect := &Rectangle{X: tileX, Y: tileY, Width: tileW, Height: tileH}
				// The Raw encoding handler will read the pixel data.
				rawEnc := &RawEncoding{}
				if err := rawEnc.Read(c, rawRect); err != nil {
					return fmt.Errorf("hextile: raw sub-encoding failed: %w", err)
				}
				continue
			}

			if subEncoding&2 != 0 { // BackgroundSpecified
				bgColor = make([]byte, bytesPerPixel)
				if _, err := io.ReadFull(c, bgColor); err != nil {
					return fmt.Errorf("hextile: failed to read background color: %w", err)
				}
			}

			if subEncoding&4 != 0 { // ForegroundSpecified
				fgColor = make([]byte, bytesPerPixel)
				if _, err := io.ReadFull(c, fgColor); err != nil {
					return fmt.Errorf("hextile: failed to read foreground color: %w", err)
				}
			}

			if subEncoding&8 != 0 { // AnySubrects
				var numSubRects uint8
				if err := binary.Read(c, binary.BigEndian, &numSubRects); err != nil {
					return fmt.Errorf("hextile: failed to read number of sub-rects: %w", err)
				}
				// Fill the tile with the background color first.
				if clientConn.Canvas != nil && bgColor != nil {
					tileRect := &Rectangle{X: tileX, Y: tileY, Width: tileW, Height: tileH}
					clientConn.Canvas.Fill(bgColor, tileRect)
				}

				for i := 0; i < int(numSubRects); i++ {
					var subRectColor []byte
					if subEncoding&16 != 0 { // SubrectsColoured
						subRectColor = make([]byte, bytesPerPixel)
						if _, err := io.ReadFull(c, subRectColor); err != nil {
							return fmt.Errorf("hextile: failed to read sub-rect color: %w", err)
						}
					} else {
						subRectColor = fgColor
					}

					var geometry [2]byte
					if _, err := io.ReadFull(c, geometry[:]); err != nil {
						return fmt.Errorf("hextile: failed to read sub-rect geometry: %w", err)
					}

					xPos := (geometry[0] >> 4) & 0x0F
					yPos := geometry[0] & 0x0F
					width := (geometry[1] >> 4) & 0x0F
					height := geometry[1] & 0x0F

					subX := tileX + uint16(xPos)
					subY := tileY + uint16(yPos)
					subW := uint16(width) + 1
					subH := uint16(height) + 1

					if clientConn.Canvas != nil && subRectColor != nil {
						sr := &Rectangle{X: subX, Y: subY, Width: subW, Height: subH}
						clientConn.Canvas.Fill(subRectColor, sr)
					}
				}
			} else { // No sub-rects, just fill the tile with the background color.
				if clientConn.Canvas != nil && bgColor != nil {
					tileRect := &Rectangle{X: tileX, Y: tileY, Width: tileW, Height: tileH}
					clientConn.Canvas.Fill(bgColor, tileRect)
				}
			}
		}
	}
	return nil
}

// Reset does nothing as this encoding is stateless.
func (e *HextileEncoding) Reset() {}
