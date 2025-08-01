package vnc2video

import (
	"errors"
	"io"
	"net"
)

// MockConn is a mock implementation of the Conn interface, used for testing.
// It allows simulating a VNC connection without actual network I/O.
type MockConn struct {
	// Reader is the source of data for the Read method.
	Reader io.Reader
	// Writer is the destination for data for the Write method.
	Writer io.Writer

	// Internal fields to hold state, corresponding to the Conn interface methods.
	pixelFormat     PixelFormat
	desktopName     []byte
	width, height   uint16
	encs            []Encoding
	protocol        string
	colorMap        ColorMap
	securityHandler SecurityHandler
}

// NewMockConn creates a new mock connection.
func NewMockConn(r io.Reader, w io.Writer, encs []Encoding) *MockConn {
	return &MockConn{
		Reader: r,
		Writer: w,
		encs:   encs,
	}
}

// Implementation of the Conn interface for MockConn.

func (m *MockConn) Read(p []byte) (n int, err error) {
	if m.Reader == nil {
		return 0, errors.New("mock reader is nil")
	}
	return m.Reader.Read(p)
}

func (m *MockConn) Write(p []byte) (n int, err error) {
	if m.Writer == nil {
		return 0, errors.New("mock writer is nil")
	}
	return m.Writer.Write(p)
}

func (m *MockConn) Close() error                      { return nil }
func (m *MockConn) Conn() net.Conn                    { return nil }
func (m *MockConn) Flush() error                      { return nil }
func (m *MockConn) Wait()                             {}
func (m *MockConn) ResetAllEncodings()                {}
func (m *MockConn) Config() interface{}               { return nil }
func (m *MockConn) SetEncodings([]EncodingType) error { return nil }
func (m *MockConn) Encodings() []Encoding             { return m.encs }
func (m *MockConn) GetEncInstance(typ EncodingType) Encoding {
	for _, enc := range m.encs {
		if enc.Type() == typ {
			return enc
		}
	}
	return nil
}

// Methods to get and set state, satisfying the Conn interface.
func (m *MockConn) PixelFormat() PixelFormat                    { return m.pixelFormat }
func (m *MockConn) SetPixelFormat(pf PixelFormat) error         { m.pixelFormat = pf; return nil }
func (m *MockConn) DesktopName() []byte                         { return m.desktopName }
func (m *MockConn) SetDesktopName(b []byte)                     { m.desktopName = b }
func (m *MockConn) Width() uint16                               { return m.width }
func (m *MockConn) SetWidth(w uint16)                           { m.width = w }
func (m *MockConn) Height() uint16                              { return m.height }
func (m *MockConn) SetHeight(h uint16)                          { m.height = h }
func (m *MockConn) Protocol() string                            { return m.protocol }
func (m *MockConn) SetProtoVersion(p string)                    { m.protocol = p }
func (m *MockConn) ColorMap() ColorMap                          { return m.colorMap }
func (m *MockConn) SetColorMap(cm ColorMap)                     { m.colorMap = cm }
func (m *MockConn) SecurityHandler() SecurityHandler            { return m.securityHandler }
func (m *MockConn) SetSecurityHandler(sh SecurityHandler) error { m.securityHandler = sh; return nil }
