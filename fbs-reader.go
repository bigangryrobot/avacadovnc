package vnc2video

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// FbsReader reads a Frame Buffer Stream (FBS) file, which is a recording
// of a VNC session.
type FbsReader struct {
	file        *os.File
	pixelFormat PixelFormat
	width       uint16
	height      uint16
	desktopName []byte

	// Internal buffer for the current data chunk being read.
	chunk []byte
}

// NewFbsReader opens and initializes a reader for the given FBS file.
// It reads the header to populate the session's metadata.
func NewFbsReader(filename string) (*FbsReader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("fbs-reader: failed to open file: %w", err)
	}

	reader := &FbsReader{file: file}

	// Read the FBS header.
	if err := binary.Read(file, binary.BigEndian, &reader.pixelFormat); err != nil {
		return nil, fmt.Errorf("fbs-reader: failed to read pixel format: %w", err)
	}
	if err := binary.Read(file, binary.BigEndian, &reader.width); err != nil {
		return nil, fmt.Errorf("fbs-reader: failed to read width: %w", err)
	}
	if err := binary.Read(file, binary.BigEndian, &reader.height); err != nil {
		return nil, fmt.Errorf("fbs-reader: failed to read height: %w", err)
	}
	var nameLen uint32
	if err := binary.Read(file, binary.BigEndian, &nameLen); err != nil {
		return nil, fmt.Errorf("fbs-reader: failed to read name length: %w", err)
	}
	reader.desktopName = make([]byte, nameLen)
	if _, err := io.ReadFull(file, reader.desktopName); err != nil {
		return nil, fmt.Errorf("fbs-reader: failed to read desktop name: %w", err)
	}

	return reader, nil
}

// Read implements the io.Reader interface. It reads the next data chunk
// from the FBS file into the provided buffer.
func (r *FbsReader) Read(p []byte) (n int, err error) {
	// If the internal chunk buffer is empty, read the next chunk from the file.
	if len(r.chunk) == 0 {
		var chunkSize uint32
		if err := binary.Read(r.file, binary.BigEndian, &chunkSize); err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.EOF // Clean end-of-file.
			}
			return 0, fmt.Errorf("fbs-reader: failed to read chunk size: %w", err)
		}

		r.chunk = make([]byte, chunkSize)
		if _, err := io.ReadFull(r.file, r.chunk); err != nil {
			return 0, fmt.Errorf("fbs-reader: failed to read chunk data: %w", err)
		}
	}

	// Copy data from the internal chunk buffer to the destination buffer `p`.
	n = copy(p, r.chunk)
	// Slice the chunk to reflect the bytes that have been "read".
	r.chunk = r.chunk[n:]

	return n, nil
}

// Close closes the underlying file.
func (r *FbsReader) Close() error {
	return r.file.Close()
}

// PixelFormat returns the pixel format from the FBS header.
func (r *FbsReader) PixelFormat() PixelFormat {
	return r.pixelFormat
}

// Width returns the framebuffer width from the FBS header.
func (r *FbsReader) Width() uint16 {
	return r.width
}

// Height returns the framebuffer height from the FBS header.
func (r *FbsReader) Height() uint16 {
	return r.height
}

// DesktopName returns the desktop name from the FBS header.
func (r *FbsReader) DesktopName() []byte {
	return r.desktopName
}
