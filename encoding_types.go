package avacadovnc

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"io"
	"net"
)

// --- Core Interfaces ---

// Handler is an interface for protocol handshake steps.
type Handler interface {
	Handle(c Conn) error
}

// Conn represents a VNC connection, abstracting the underlying net.Conn.
type Conn interface {
	io.Reader
	io.Writer
	io.Closer
	Conn() net.Conn
	ColorMap() ColorMap
	SetColorMap(ColorMap)
	DesktopName() []byte
	SetDesktopName([]byte)
	Encodings() []Encoding
	GetEncInstance(typ EncodingType) Encoding
	SetEncodings(encs []EncodingType) error
	ResetAllEncodings()
	Flush() error
	PixelFormat() PixelFormat
	SetPixelFormat(PixelFormat) error
	Protocol() string
	SetProtoVersion(string)
	SecurityHandler() SecurityHandler
	SetSecurityHandler(sechandler SecurityHandler) error
	Width() uint16
	SetWidth(uint16)
	Height() uint16
	SetHeight(uint16)
	Config() interface{}
}

// SecurityHandler defines the interface for a VNC security scheme.
type SecurityHandler interface {
	Type() SecurityType
	Authenticate(c Conn) error
}

// Encoding defines the interface for a VNC encoding handler.
type Encoding interface {
	Type() EncodingType
	Read(c Conn, rect *Rectangle) error
	Reset()
}

// ClientMessage defines the interface for messages sent from the client to the server.
type ClientMessage interface {
	Type() ClientMessageType
	Write(c Conn) error
}

// ServerMessage defines the interface for messages sent from the server to the client.
type ServerMessage interface {
	Type() ServerMessageType
	Read(c Conn) (ServerMessage, error)
}

// --- Core Structs & Types ---

type Rectangle struct {
	X, Y          uint16
	Width, Height uint16
}

var DefaultPixelFormat = PixelFormat{
	BPP: 32, Depth: 24, BigEndian: 0, TrueColor: 1,
	RedMax: 255, GreenMax: 255, BlueMax: 255,
	RedShift: 16, GreenShift: 8, BlueShift: 0,
}

type PixelFormat struct {
	BPP, Depth, BigEndian, TrueColor uint8
	RedMax, GreenMax, BlueMax        uint16
	RedShift, GreenShift, BlueShift  uint8
	_                                [3]byte
}

func (pf *PixelFormat) BytesPerPixel() int {
	return int(pf.BPP / 8)
}

type ColorMap [256]color.RGBA
type ButtonMask uint8
type Key uint32

type ClientConfig struct {
	Handlers         []Handler
	SecurityHandlers []SecurityHandler
	Encodings        []Encoding
	PixelFormat      PixelFormat
	ColorMap         ColorMap
	ClientMessageCh  chan ClientMessage
	ServerMessageCh  chan ServerMessage
	Exclusive        bool
	DrawCursor       bool
	Messages         []ServerMessage
}

type ServerConfig struct {
	Handlers         []Handler
	SecurityHandlers []SecurityHandler
	Encodings        []Encoding
	PixelFormat      PixelFormat
	Width, Height    uint16
	DesktopName      string
	quit             chan struct{}
}

// --- Enumerations and Stringers ---
type EncodingType int32

const (
	EncRaw         EncodingType = 0
	EncCopyRect    EncodingType = 1
	EncRRE         EncodingType = 2
	EncCoRRE       EncodingType = 4
	EncHextile     EncodingType = 5
	EncZlib        EncodingType = 6
	EncTight       EncodingType = 7
	EncZRLE        EncodingType = 16
	EncTightPNG    EncodingType = -260
	EncDesktopSize EncodingType = -223
	EncLastRect    EncodingType = -224
	EncCursor      EncodingType = -239
	EncXCursor     EncodingType = -240
	EncAtenHermon  EncodingType = -305
	EncDesktopName EncodingType = -307
	EncPointerPos  EncodingType = -258
)

type ClientMessageType uint8

const (
	ClientSetPixelFormat           ClientMessageType = 0
	ClientSetEncodings             ClientMessageType = 2
	ClientFramebufferUpdateRequest ClientMessageType = 3
	ClientKeyEvent                 ClientMessageType = 4
	ClientPointerEvent             ClientMessageType = 5
	ClientCutText                  ClientMessageType = 6
)

type ServerMessageType uint8

const (
	ServerFramebufferUpdate   ServerMessageType = 0
	ServerSetColourMapEntries ServerMessageType = 1
	ServerBell                ServerMessageType = 2
	ServerCutText             ServerMessageType = 3
)

type SecurityType uint8

const (
	SecTypeInvalid      SecurityType = 0
	SecTypeNone         SecurityType = 1
	SecTypeVNCAuth      SecurityType = 2
	SecTypeTight        SecurityType = 16
	SecTypeVeNCrypt     SecurityType = 19
	SecTypeAtenHermon   SecurityType = 20
	SecTypeAtenUltraVNC SecurityType = 21
	SecTypeAtenTLS      SecurityType = 22
	SecTypeAtenSASL     SecurityType = 23
	SecTypeAtenXVP      SecurityType = 24
)

// --- Client-to-Server Messages ---

type SetPixelFormat struct{ PixelFormat }

func (m *SetPixelFormat) Type() ClientMessageType { return ClientSetPixelFormat }
func (m *SetPixelFormat) Write(c Conn) error {
	buf := []byte{byte(ClientSetPixelFormat), 0, 0, 0}
	if _, err := c.Write(buf); err != nil {
		return err
	}
	return binary.Write(c, binary.BigEndian, m.PixelFormat)
}

type SetEncodings struct{ Encodings []EncodingType }

func (m *SetEncodings) Type() ClientMessageType { return ClientSetEncodings }
func (m *SetEncodings) Write(c Conn) error {
	if err := binary.Write(c, binary.BigEndian, ClientSetEncodings); err != nil {
		return err
	}
	if err := binary.Write(c, binary.BigEndian, [1]byte{}); err != nil {
		return err
	}
	if err := binary.Write(c, binary.BigEndian, uint16(len(m.Encodings))); err != nil {
		return err
	}
	return binary.Write(c, binary.BigEndian, m.Encodings)
}

type FramebufferUpdateRequest struct {
	Inc                 uint8
	X, Y, Width, Height uint16
}

func (m *FramebufferUpdateRequest) Type() ClientMessageType { return ClientFramebufferUpdateRequest }
func (m *FramebufferUpdateRequest) Write(c Conn) error {
	buf := []byte{byte(ClientFramebufferUpdateRequest), m.Inc, byte(m.X >> 8), byte(m.X), byte(m.Y >> 8), byte(m.Y), byte(m.Width >> 8), byte(m.Width), byte(m.Height >> 8), byte(m.Height)}
	_, err := c.Write(buf)
	return err
}

type KeyEvent struct {
	Down uint8
	Key  Key
}

func (m *KeyEvent) Type() ClientMessageType { return ClientKeyEvent }
func (m *KeyEvent) Write(c Conn) error {
	buf := []byte{byte(ClientKeyEvent), m.Down, 0, 0, byte(m.Key >> 24), byte(m.Key >> 16), byte(m.Key >> 8), byte(m.Key)}
	_, err := c.Write(buf)
	return err
}

type PointerEvent struct {
	Mask ButtonMask
	X, Y uint16
}

func (m *PointerEvent) Type() ClientMessageType { return ClientPointerEvent }
func (m *PointerEvent) Write(c Conn) error {
	buf := []byte{byte(ClientPointerEvent), byte(m.Mask), byte(m.X >> 8), byte(m.X), byte(m.Y >> 8), byte(m.Y)}
	_, err := c.Write(buf)
	return err
}

type CutText struct{ Text []byte }

func (m *CutText) Type() ClientMessageType { return ClientCutText }
func (m *CutText) Write(c Conn) error {
	if err := binary.Write(c, binary.BigEndian, ClientCutText); err != nil {
		return err
	}
	if err := binary.Write(c, binary.BigEndian, [3]byte{}); err != nil {
		return err
	}
	if err := binary.Write(c, binary.BigEndian, uint32(len(m.Text))); err != nil {
		return err
	}
	_, err := c.Write(m.Text)
	return err
}

// --- Server-to-Client Messages ---

type FramebufferUpdateMessage struct{}

func (m *FramebufferUpdateMessage) Type() ServerMessageType { return ServerFramebufferUpdate }
func (m *FramebufferUpdateMessage) Read(c Conn) (ServerMessage, error) {
	var padding [1]byte
	if _, err := io.ReadFull(c, padding[:]); err != nil {
		return nil, err
	}
	var numRects uint16
	if err := binary.Read(c, binary.BigEndian, &numRects); err != nil {
		return nil, err
	}
	for i := uint16(0); i < numRects; i++ {
		var rect Rectangle
		var encodingType EncodingType
		if err := binary.Read(c, binary.BigEndian, &rect); err != nil {
			return nil, err
		}
		if err := binary.Read(c, binary.BigEndian, &encodingType); err != nil {
			return nil, err
		}
		enc := c.GetEncInstance(encodingType)
		if enc == nil {
			return nil, fmt.Errorf("unsupported encoding: %v", encodingType)
		}
		if err := enc.Read(c, &rect); err != nil {
			return nil, err
		}
	}
	return m, nil
}

type SetColourMapEntriesMessage struct{}

func (m *SetColourMapEntriesMessage) Type() ServerMessageType { return ServerSetColourMapEntries }
func (m *SetColourMapEntriesMessage) Read(c Conn) (ServerMessage, error) {
	var padding [1]byte
	if _, err := io.ReadFull(c, padding[:]); err != nil {
		return nil, err
	}
	var firstColor, numColors uint16
	if err := binary.Read(c, binary.BigEndian, &firstColor); err != nil {
		return nil, err
	}
	if err := binary.Read(c, binary.BigEndian, &numColors); err != nil {
		return nil, err
	}
	cm := c.ColorMap()
	for i := 0; i < int(numColors); i++ {
		var r, g, b uint16
		if err := binary.Read(c, binary.BigEndian, &r); err != nil {
			return nil, err
		}
		if err := binary.Read(c, binary.BigEndian, &g); err != nil {
			return nil, err
		}
		if err := binary.Read(c, binary.BigEndian, &b); err != nil {
			return nil, err
		}
		cm[int(firstColor)+i] = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
	}
	c.SetColorMap(cm)
	return m, nil
}

type ServerBellMessage struct{}

func (m *ServerBellMessage) Type() ServerMessageType            { return ServerBell }
func (m *ServerBellMessage) Read(c Conn) (ServerMessage, error) { return m, nil }

type ServerCutTextMessage struct{ Text []byte }

func (m *ServerCutTextMessage) Type() ServerMessageType { return ServerCutText }
func (m *ServerCutTextMessage) Read(c Conn) (ServerMessage, error) {
	var padding [3]byte
	if _, err := io.ReadFull(c, padding[:]); err != nil {
		return nil, err
	}
	var length uint32
	if err := binary.Read(c, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	m.Text = make([]byte, length)
	if _, err := io.ReadFull(c, m.Text); err != nil {
		return nil, err
	}
	return m, nil
}
