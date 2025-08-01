package vnc2video

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
)

// FbsConnection represents a connection that records the VNC stream to a
// Frame Buffer Stream (FBS) file. It wraps a standard net.Conn.
type FbsConnection struct {
	net.Conn
	file *os.File
}

// NewFbsConnection creates a new recording connection.
func NewFbsConnection(conn net.Conn, file *os.File) (*FbsConnection, error) {
	if file == nil {
		return nil, errors.New("fbs-connection: file cannot be nil")
	}
	return &FbsConnection{
		Conn: conn,
		file: file,
	}, nil
}

// Read reads data from the underlying connection and writes it to the FBS file.
func (fbs *FbsConnection) Read(b []byte) (n int, err error) {
	n, err = fbs.Conn.Read(b)
	if n > 0 {
		// Write the chunk size followed by the data.
		if errw := binary.Write(fbs.file, binary.BigEndian, uint32(n)); errw != nil {
			return n, fmt.Errorf("fbs-connection: failed to write chunk size: %w", errw)
		}
		if _, errw := fbs.file.Write(b[0:n]); errw != nil {
			return n, fmt.Errorf("fbs-connection: failed to write chunk data: %w", errw)
		}
	}
	return n, err
}

// Write writes data to the underlying connection. It does not record outgoing data.
func (fbs *FbsConnection) Write(b []byte) (n int, err error) {
	return fbs.Conn.Write(b)
}

// Close closes both the network connection and the file.
func (fbs *FbsConnection) Close() error {
	fbs.file.Close()
	return fbs.Conn.Close()
}

// FbsStreamer is a utility to record a VNC session to an FBS file.
type FbsStreamer struct {
	clientConn *ClientConn
	file       *os.File
}

// NewFbsStreamer creates a new streamer.
func NewFbsStreamer(cc *ClientConn, f *os.File) *FbsStreamer {
	return &FbsStreamer{
		clientConn: cc,
		file:       f,
	}
}

// RecordSession starts the recording process.
// This function should be called after a successful VNC handshake.
func (s *FbsStreamer) RecordSession() error {
	// Write the FBS header with the server's initial parameters.
	// This data is now read from the ClientConn object, not a ServerInit message.
	pf := s.clientConn.PixelFormat()
	width := s.clientConn.Width()
	height := s.clientConn.Height()
	name := s.clientConn.DesktopName()

	// Write pixel format
	if err := binary.Write(s.file, binary.BigEndian, &pf); err != nil {
		return fmt.Errorf("fbs-streamer: failed to write pixel format: %w", err)
	}
	// Write screen dimensions
	if err := binary.Write(s.file, binary.BigEndian, &width); err != nil {
		return fmt.Errorf("fbs-streamer: failed to write width: %w", err)
	}
	if err := binary.Write(s.file, binary.BigEndian, &height); err != nil {
		return fmt.Errorf("fbs-streamer: failed to write height: %w", err)
	}
	// Write desktop name
	nameLen := uint32(len(name))
	if err := binary.Write(s.file, binary.BigEndian, &nameLen); err != nil {
		return fmt.Errorf("fbs-streamer: failed to write name length: %w", err)
	}
	if _, err := s.file.Write(name); err != nil {
		return fmt.Errorf("fbs-streamer: failed to write name: %w", err)
	}

	// The rest of the data is read from the connection and written to the file
	// by the FbsConnection's Read method. We just need to drain the reader.
	_, err := io.Copy(io.Discard, bufio.NewReader(s.clientConn.Conn()))
	return err
}
