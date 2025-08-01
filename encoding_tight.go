package avacadovnc

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/bigangryrobot/avacadovnc/logger"
)

// TightEncoding implements the Tight VNC encoding, a highly efficient encoding
// that uses zlib compression and various filters to reduce bandwidth.
type TightEncoding struct {
	// zlibs holds the zlib reader streams. The protocol allows for up to 4
	// separate streams to be used for different types of data.
	zlibs [4]io.ReadCloser
	// buffer is a reusable buffer for reading compressed data, to reduce allocations.
	buffer *bytes.Buffer
}

// Type returns the encoding type identifier.
func (e *TightEncoding) Type() EncodingType {
	return EncTight
}

// Read decodes a rectangle of pixel data using the Tight encoding.
func (e *TightEncoding) Read(c Conn, rect *Rectangle) error {
	// The first byte is the compression control byte. It determines which
	// zlib streams to reset and which sub-encoding (filter) to use.
	var compControl [1]byte
	if _, err := io.ReadFull(c, compControl[:]); err != nil {
		return fmt.Errorf("tight: failed to read compression control: %w", err)
	}

	// Bits 0-3 of compControl indicate which zlib streams should be reset.
	for i := 0; i < 4; i++ {
		if (compControl[0]>>i)&1 != 0 {
			if e.zlibs[i] != nil {
				e.zlibs[i].Close()
				e.zlibs[i] = nil
			}
		}
	}

	// Dispatch to the correct sub-encoding handler based on the compControl byte.
	if compControl[0]&0x80 == 0 {
		// Bit 7 is 0: Basic compression (Copy, Palette, Gradient, or plain zlib).
		streamID := (compControl[0] >> 4) & 0x03
		filterID := compControl[0] & 0x70

		switch filterID {
		case 0x40: // Gradient filter
			return e.handleGradient(c, rect)
		case 0x20: // Palette filter
			return e.handlePalette(c, rect, streamID)
		case 0x10, 0x00: // Copy (plain zlib)
			return e.handleCopy(c, rect, streamID)
		default:
			return fmt.Errorf("tight: unsupported basic filter: %x", filterID)
		}
	}

	// Bit 7 is 1: Fill, JPEG, or PNG compression.
	switch compControl[0] & 0xF0 {
	case 0x80: // Fill compression
		return e.handleFill(c, rect)
	case 0x90: // JPEG compression
		return e.handleJPEG(c, rect)
	case 0xA0: // PNG compression
		return e.handlePNG(c, rect)
	default:
		return fmt.Errorf("tight: unsupported compression control value: %x", compControl[0])
	}
}

// handleCopy decodes raw pixel data compressed with zlib.
func (e *TightEncoding) handleCopy(c Conn, rect *Rectangle, streamID byte) error {
	bytesPerPixel := c.PixelFormat().BPP
	rowSize := int(rect.Width) * int(bytesPerPixel)
	uncompressedSize := rowSize * int(rect.Height)

	compressedData, err := e.readCompressedData(c)
	if err != nil {
		return err
	}
	if len(compressedData) == 0 {
		return nil // No data to process.
	}

	// Decompress the data using the appropriate zlib stream.
	pixelData, err := e.decompress(compressedData, uncompressedSize, streamID)
	if err != nil {
		return err
	}

	// Draw the raw pixel data to the canvas.
	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil // No canvas to draw on.
	}
	return clientConn.Canvas.DrawBytes(pixelData, rect)
}

// handleFill decodes a rectangle filled with a single color.
func (e *TightEncoding) handleFill(c Conn, rect *Rectangle) error {
	bytesPerPixel := c.PixelFormat().BPP
	colorBytes := make([]byte, bytesPerPixel)
	if _, err := io.ReadFull(c, colorBytes); err != nil {
		return fmt.Errorf("tight: failed to read fill color: %w", err)
	}

	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil // No canvas to draw on.
	}
	return clientConn.Canvas.Fill(colorBytes, rect)
}

// handleJPEG decodes a JPEG-encoded rectangle.
func (e *TightEncoding) handleJPEG(c Conn, rect *Rectangle) error {
	jpegData, err := e.readCompressedData(c)
	if err != nil {
		return err
	}
	if len(jpegData) == 0 {
		return nil
	}

	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return fmt.Errorf("tight: failed to decode jpeg: %w", err)
	}

	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil
	}
	clientConn.Canvas.Draw(img, rect)
	return nil
}

// handlePNG is a placeholder for PNG-compressed rectangles.
func (e *TightEncoding) handlePNG(c Conn, rect *Rectangle) error {
	pngData, err := e.readCompressedData(c)
	if err != nil {
		return err
	}
	if len(pngData) == 0 {
		return nil
	}

	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return fmt.Errorf("tight: failed to decode png: %w", err)
	}

	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil
	}
	clientConn.Canvas.Draw(img, rect)
	return nil
}

// handlePalette decodes indexed color data.
func (e *TightEncoding) handlePalette(c Conn, rect *Rectangle, streamID byte) error {
	var numColors [1]byte
	if _, err := io.ReadFull(c, numColors[:]); err != nil {
		return fmt.Errorf("tight: failed to read palette size: %w", err)
	}
	paletteSize := int(numColors[0]) + 1
	bytesPerPixel := c.PixelFormat().BPP

	// Read the palette.
	paletteData := make([]byte, paletteSize*int(bytesPerPixel))
	if _, err := io.ReadFull(c, paletteData); err != nil {
		return fmt.Errorf("tight: failed to read palette data: %w", err)
	}

	// Determine if the indexed data is 1-bit or 8-bit.
	var bitsPerIndex int
	var rowSize int
	if paletteSize <= 2 {
		bitsPerIndex = 1
		rowSize = (int(rect.Width) + 7) / 8
	} else {
		bitsPerIndex = 8
		rowSize = int(rect.Width)
	}
	uncompressedSize := rowSize * int(rect.Height)

	// Decompress the indexed data.
	compressedData, err := e.readCompressedData(c)
	if err != nil {
		return err
	}
	indexedData, err := e.decompress(compressedData, uncompressedSize, streamID)
	if err != nil {
		return err
	}

	// Convert indexed data to full color and draw.
	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil
	}
	return clientConn.Canvas.DrawPalette(indexedData, paletteData, bitsPerIndex, rect)
}

// handleGradient is a placeholder for gradient-filled rectangles.
// This is rarely used in practice, so we log and skip it.
func (e *TightEncoding) handleGradient(c Conn, rect *Rectangle) error {
	logger.Warn("tight: gradient filter is not implemented, skipping rectangle")
	// Gradient data is uncompressed raw pixel data.
	bytesToRead := int(rect.Width) * int(rect.Height) * int(c.PixelFormat().BPP)
	if _, err := io.CopyN(io.Discard, c, int64(bytesToRead)); err != nil {
		return fmt.Errorf("tight: failed to discard gradient data: %w", err)
	}
	return nil
}

// decompress handles the zlib decompression.
func (e *TightEncoding) decompress(data []byte, uncompressedSize int, streamID byte) ([]byte, error) {
	if e.zlibs[streamID] == nil {
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("tight: failed to create zlib reader: %w", err)
		}
		e.zlibs[streamID] = zr
	} else {
		if resetter, ok := e.zlibs[streamID].(zlib.Resetter); ok {
			err := resetter.Reset(bytes.NewReader(data), nil)
			if err != nil {
				return nil, fmt.Errorf("tight: failed to reset zlib reader: %w", err)
			}
		} else {
			e.zlibs[streamID].Close()
			zr, err := zlib.NewReader(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("tight: failed to create new zlib reader: %w", err)
			}
			e.zlibs[streamID] = zr
		}
	}

	if e.buffer == nil {
		e.buffer = &bytes.Buffer{}
	}
	e.buffer.Reset()
	e.buffer.Grow(uncompressedSize)
	if _, err := io.CopyN(e.buffer, e.zlibs[streamID], int64(uncompressedSize)); err != nil {
		return nil, fmt.Errorf("tight: zlib decompression failed: %w", err)
	}
	return e.buffer.Bytes(), nil
}

// readCompressedData reads a compactly represented length followed by the data itself.
func (e *TightEncoding) readCompressedData(c io.Reader) ([]byte, error) {
	var b [1]byte
	if _, err := io.ReadFull(c, b[:]); err != nil {
		return nil, fmt.Errorf("tight: failed to read length byte 1: %w", err)
	}
	length := int(b[0] & 0x7F)

	if b[0]&0x80 != 0 {
		if _, err := io.ReadFull(c, b[:]); err != nil {
			return nil, fmt.Errorf("tight: failed to read length byte 2: %w", err)
		}
		length |= int(b[0]&0x7F) << 7
		if b[0]&0x80 != 0 {
			if _, err := io.ReadFull(c, b[:]); err != nil {
				return nil, fmt.Errorf("tight: failed to read length byte 3: %w", err)
			}
			length |= int(b[0]) << 14
		}
	}

	if length == 0 {
		return nil, nil
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(c, data); err != nil {
		return nil, fmt.Errorf("tight: failed to read compressed data (len=%d): %w", length, err)
	}
	return data, nil
}

// Reset cleans up the zlib streams.
func (e *TightEncoding) Reset() {
	for i := range e.zlibs {
		if e.zlibs[i] != nil {
			e.zlibs[i].Close()
			e.zlibs[i] = nil
		}
	}
	e.buffer = nil
}
