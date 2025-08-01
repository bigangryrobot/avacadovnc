package vnc2video

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image/png"
	"io"
)

// TightPNGEncoding implements the TightPNG encoding, which is a variation
// of the Tight encoding that uses PNG compression.
type TightPNGEncoding struct {
	buffer *bytes.Buffer
}

// Type returns the encoding type identifier.
func (e *TightPNGEncoding) Type() EncodingType {
	return EncTightPNG
}

// Read decodes a TightPNG-encoded rectangle.
func (e *TightPNGEncoding) Read(c Conn, rect *Rectangle) error {
	// The format is a compact length followed by zlib-compressed PNG data.
	compressedData, err := e.readCompressedData(c)
	if err != nil {
		return fmt.Errorf("tight-png: %w", err)
	}

	if len(compressedData) == 0 {
		return nil
	}

	if e.buffer == nil {
		e.buffer = &bytes.Buffer{}
	}
	e.buffer.Reset()
	e.buffer.Write(compressedData)

	zlibReader, err := zlib.NewReader(e.buffer)
	if err != nil {
		return fmt.Errorf("tight-png: failed to create zlib reader: %w", err)
	}
	defer zlibReader.Close()

	img, err := png.Decode(zlibReader)
	if err != nil {
		return fmt.Errorf("tight-png: failed to decode png: %w", err)
	}

	clientConn, ok := c.(*ClientConn)
	if !ok || clientConn.Canvas == nil {
		return nil
	}

	clientConn.Canvas.Draw(img, rect)
	return nil
}

// readCompressedData reads a compactly represented length followed by the data itself.
// This logic is shared with the main Tight encoding.
func (e *TightPNGEncoding) readCompressedData(c io.Reader) ([]byte, error) {
	var b [1]byte
	if _, err := io.ReadFull(c, b[:]); err != nil {
		return nil, fmt.Errorf("failed to read length byte 1: %w", err)
	}
	length := int(b[0] & 0x7F)

	if b[0]&0x80 != 0 {
		if _, err := io.ReadFull(c, b[:]); err != nil {
			return nil, fmt.Errorf("failed to read length byte 2: %w", err)
		}
		length |= int(b[0]&0x7F) << 7
		if b[0]&0x80 != 0 {
			if _, err := io.ReadFull(c, b[:]); err != nil {
				return nil, fmt.Errorf("failed to read length byte 3: %w", err)
			}
			length |= int(b[0]) << 14
		}
	}

	if length == 0 {
		return nil, nil
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(c, data); err != nil {
		return nil, fmt.Errorf("failed to read compressed data (len=%d): %w", length, err)
	}
	return data, nil
}

// Reset cleans up the internal buffer.
func (e *TightPNGEncoding) Reset() {
	e.buffer = nil
}
