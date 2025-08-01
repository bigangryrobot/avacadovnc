package avacadovnc

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

// ZlibEncoding implements the Zlib encoding, which sends zlib-compressed
// raw pixel data.
type ZlibEncoding struct {
	zlibReader io.ReadCloser
	buffer     *bytes.Buffer
}

// Type returns the encoding type identifier.
func (e *ZlibEncoding) Type() EncodingType {
	return EncZlib
}

// Read decodes a zlib-compressed rectangle.
func (e *ZlibEncoding) Read(c Conn, rect *Rectangle) error {
	var compressedLen uint32
	if err := binary.Read(c, binary.BigEndian, &compressedLen); err != nil {
		return fmt.Errorf("zlib: failed to read compressed data length: %w", err)
	}

	if compressedLen == 0 {
		return nil
	}

	compressedData := make([]byte, compressedLen)
	if _, err := io.ReadFull(c, compressedData); err != nil {
		return fmt.Errorf("zlib: failed to read compressed data: %w", err)
	}

	if e.zlibReader == nil {
		var err error
		e.zlibReader, err = zlib.NewReader(bytes.NewReader(compressedData))
		if err != nil {
			return fmt.Errorf("zlib: failed to create zlib reader: %w", err)
		}
	} else {
		// The zlib.Resetter interface allows us to reuse the reader.
		if resetter, ok := e.zlibReader.(zlib.Resetter); ok {
			if err := resetter.Reset(bytes.NewReader(compressedData), nil); err != nil {
				return fmt.Errorf("zlib: failed to reset zlib reader: %w", err)
			}
		} else {
			// Fallback for older zlib versions or different implementations.
			e.zlibReader.Close()
			var err error
			e.zlibReader, err = zlib.NewReader(bytes.NewReader(compressedData))
			if err != nil {
				return fmt.Errorf("zlib: failed to create new zlib reader: %w", err)
			}
		}
	}

	// Calculate the size of the uncompressed pixel data.
	bytesPerPixel := c.PixelFormat().BPP
	uncompressedSize := int(rect.Width) * int(rect.Height) * int(bytesPerPixel)

	// Read the decompressed raw pixel data.
	pixelData := make([]byte, uncompressedSize)
	if _, err := io.ReadFull(e.zlibReader, pixelData); err != nil {
		return fmt.Errorf("zlib: failed to decompress pixel data: %w", err)
	}

	// Draw the decoded bytes to the canvas.
	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil // No canvas to draw on.
	}

	return clientConn.Canvas.DrawBytes(pixelData, rect)
}

// Reset cleans up the zlib reader.
func (e *ZlibEncoding) Reset() {
	if e.zlibReader != nil {
		e.zlibReader.Close()
		e.zlibReader = nil
	}
}
