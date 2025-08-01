package avacadovnc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/draw"
	"io"
	"net"

	"github.com/bigangryrobot/avacadovnc/logger"
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

type ClientMessage interface {
	String() string
	Type() ClientMessageType
	Read(Conn) (ClientMessage, error)
	Write(Conn) error
	Supported(Conn) bool
}

type ServerMessage interface {
	String() string
	Type() ServerMessageType
	Read(Conn) (ServerMessage, error)
	Write(Conn) error
	Supported(Conn) bool
}

// --- Core Structs & Types ---

// Rectangle represents a rectangle of pixel data
type Rectangle struct {
	X, Y          uint16
	Width, Height uint16
	EncType       EncodingType
	Enc           Encoding
}

// String return string representation
func (rect *Rectangle) String() string {
	return fmt.Sprintf("rect x: %d, y: %d, width: %d, height: %d, enc: %s", rect.X, rect.Y, rect.Width, rect.Height, rect.EncType)
}

// NewRectangle returns new rectangle
func NewRectangle() *Rectangle {
	return &Rectangle{}
}

// // Write marshal color to conn
// func (clr *Color) Write(c Conn) error {
// 	var err error
// 	pf := c.PixelFormat()
// 	order := pf.order()
// 	pixel := clr.cmIndex
// 	if clr.pf.TrueColor != 0 {
// 		pixel = uint32(clr.R) << pf.RedShift
// 		pixel |= uint32(clr.G) << pf.GreenShift
// 		pixel |= uint32(clr.B) << pf.BlueShift
// 	}

// 	switch pf.BPP {
// 	case 8:
// 		err = binary.Write(c, order, byte(pixel))
// 	case 16:
// 		err = binary.Write(c, order, uint16(pixel))
// 	case 32:
// 		err = binary.Write(c, order, uint32(pixel))
// 	}

// 	return err
// }

// // Read unmarshal color from conn
// func (clr *Color) Read(c Conn) error {
// 	order := clr.pf.order()
// 	var pixel uint32

// 	switch clr.pf.BPP {
// 	case 8:
// 		var px uint8
// 		if err := binary.Read(c, order, &px); err != nil {
// 			return err
// 		}
// 		pixel = uint32(px)
// 	case 16:
// 		var px uint16
// 		if err := binary.Read(c, order, &px); err != nil {
// 			return err
// 		}
// 		pixel = uint32(px)
// 	case 32:
// 		var px uint32
// 		if err := binary.Read(c, order, &px); err != nil {
// 			return err
// 		}
// 		pixel = uint32(px)
// 	}

// 	if clr.pf.TrueColor != 0 {
// 		clr.R = uint16((pixel >> clr.pf.RedShift) & uint32(clr.pf.RedMax))
// 		clr.G = uint16((pixel >> clr.pf.GreenShift) & uint32(clr.pf.GreenMax))
// 		clr.B = uint16((pixel >> clr.pf.BlueShift) & uint32(clr.pf.BlueMax))
// 	} else {
// 		*clr = clr.cm[pixel]
// 		clr.cmIndex = pixel
// 	}
// 	return nil
// }

// func colorsToImage(x, y, width, height uint16, colors []Color) *image.RGBA64 {
// 	rect := image.Rect(int(x), int(y), int(x+width), int(y+height))
// 	rgba := image.NewRGBA64(rect)
// 	a := uint16(1)
// 	for i, color := range colors {
// 		rgba.Pix[4*i+0] = uint8(color.R >> 8)
// 		rgba.Pix[4*i+1] = uint8(color.R)
// 		rgba.Pix[4*i+2] = uint8(color.G >> 8)
// 		rgba.Pix[4*i+3] = uint8(color.G)
// 		rgba.Pix[4*i+4] = uint8(color.B >> 8)
// 		rgba.Pix[4*i+5] = uint8(color.B)
// 		rgba.Pix[4*i+6] = uint8(a >> 8)
// 		rgba.Pix[4*i+7] = uint8(a)
// 	}
// 	return rgba
// }

// Write marshal rectangle to conn
func (rect *Rectangle) Write(c Conn) error {
	var err error

	if err = binary.Write(c, binary.BigEndian, rect.X); err != nil {
		return err
	}
	if err = binary.Write(c, binary.BigEndian, rect.Y); err != nil {
		return err
	}
	if err = binary.Write(c, binary.BigEndian, rect.Width); err != nil {
		return err
	}
	if err = binary.Write(c, binary.BigEndian, rect.Height); err != nil {
		return err
	}
	if err = binary.Write(c, binary.BigEndian, rect.EncType); err != nil {
		return err
	}

	return rect.Write(c)
}

// Read unmarshal rectangle from conn
func (rect *Rectangle) Read(c Conn) error {
	var err error

	if err = binary.Read(c, binary.BigEndian, &rect.X); err != nil {
		return err
	}
	if err = binary.Read(c, binary.BigEndian, &rect.Y); err != nil {
		return err
	}
	if err = binary.Read(c, binary.BigEndian, &rect.Width); err != nil {
		return err
	}
	if err = binary.Read(c, binary.BigEndian, &rect.Height); err != nil {
		return err
	}
	if err = binary.Read(c, binary.BigEndian, &rect.EncType); err != nil {
		return err
	}
	logger.Debug(rect)
	switch rect.EncType {
	// case EncCopyRect:
	// 	rect.Enc = &CopyRectEncoding{}
	// case EncTight:
	// 	rect.Enc = c.GetEncInstance(rect.EncType)
	// case EncTightPng:
	// 	rect.Enc = &TightPngEncoding{}
	// case EncRaw:
	// 	if strings.HasPrefix(c.Protocol(), "aten") {
	// 		rect.Enc = &AtenHermon{}
	// 	} else {
	// 		rect.Enc = &RawEncoding{}
	// 	}
	case EncDesktopSize:
		rect.Enc = &DesktopSizeEncoding{}
	case EncDesktopName:
		rect.Enc = &DesktopNameEncoding{}
	// case EncXCursorPseudo:
	// 	rect.Enc = &XCursorPseudoEncoding{}
	// case EncAtenHermon:
	// 	rect.Enc = &AtenHermon{}
	default:
		rect.Enc = c.GetEncInstance(rect.EncType)
		if rect.Enc == nil {
			return fmt.Errorf("unsupported encoding %s", rect.EncType)
		}
	}

	return rect.Enc.Read(c, rect)
}

// Area returns the total area in pixels of the Rectangle
func (rect *Rectangle) Area() int { return int(rect.Width) * int(rect.Height) }

var DefaultPixelFormat = PixelFormat{
	BPP: 32, Depth: 24, BigEndian: 0, TrueColor: 1,
	RedMax: 255, GreenMax: 255, BlueMax: 255,
	RedShift: 16, GreenShift: 8, BlueShift: 0,
}

// PixelFormat describes the way a pixel is formatted for a VNC connection
type PixelFormat struct {
	BPP                             uint8   // bits-per-pixel
	Depth                           uint8   // depth
	BigEndian                       uint8   // big-endian-flag
	TrueColor                       uint8   // true-color-flag
	RedMax, GreenMax, BlueMax       uint16  // red-, green-, blue-max (2^BPP-1)
	RedShift, GreenShift, BlueShift uint8   // red-, green-, blue-shift
	_                               [3]byte // padding
}

const pixelFormatLen = 16

// NewPixelFormat returns a populated PixelFormat structure
func NewPixelFormat(bpp uint8) PixelFormat {
	bigEndian := uint8(0)
	//	rgbMax := uint16(math.Exp2(float64(bpp))) - 1
	rMax := uint16(255)
	gMax := uint16(255)
	bMax := uint16(255)
	var (
		tc         = uint8(1)
		rs, gs, bs uint8
		depth      uint8
	)
	switch bpp {
	case 8:
		tc = 0
		depth = 8
		rs, gs, bs = 0, 0, 0
	case 16:
		depth = 16
		rs, gs, bs = 0, 4, 8
	case 32:
		depth = 24
		//	rs, gs, bs = 0, 8, 16
		rs, gs, bs = 16, 8, 0
	}
	return PixelFormat{bpp, depth, bigEndian, tc, rMax, gMax, bMax, rs, gs, bs, [3]byte{}}
}

// NewPixelFormatAten returns Aten IKVM pixel format
func NewPixelFormatAten() PixelFormat {
	return PixelFormat{16, 15, 0, 1, (1 << 5) - 1, (1 << 5) - 1, (1 << 5) - 1, 10, 5, 0, [3]byte{}}
}

// Marshal implements the Marshaler interface
func (pf PixelFormat) Marshal() ([]byte, error) {
	// Validation checks.
	switch pf.BPP {
	case 8, 16, 32:
	default:
		return nil, fmt.Errorf("Invalid BPP value %v; must be 8, 16, or 32", pf.BPP)
	}

	if pf.Depth < pf.BPP {
		return nil, fmt.Errorf("Invalid Depth value %v; cannot be < BPP", pf.Depth)
	}
	switch pf.Depth {
	case 8, 16, 32:
	default:
		return nil, fmt.Errorf("Invalid Depth value %v; must be 8, 16, or 32", pf.Depth)
	}

	// Create the slice of bytes
	buf := bPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bPool.Put(buf)

	if err := binary.Write(buf, binary.BigEndian, &pf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Read reads from an io.Reader, and populates the PixelFormat
func (pf PixelFormat) Read(r io.Reader) error {
	buf := make([]byte, pixelFormatLen)
	if _, err := io.ReadAtLeast(r, buf, pixelFormatLen); err != nil {
		return err
	}
	return pf.Unmarshal(buf)
}

// Unmarshal implements the Unmarshaler interface
func (pf PixelFormat) Unmarshal(data []byte) error {
	buf := bPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bPool.Put(buf)

	if _, err := buf.Write(data); err != nil {
		return err
	}

	if err := binary.Read(buf, binary.BigEndian, &pf); err != nil {
		return err
	}

	return nil
}

// String implements the fmt.Stringer interface
func (pf PixelFormat) String() string {
	return fmt.Sprintf("{ bpp: %d depth: %d big-endian: %d true-color: %d red-max: %d green-max: %d blue-max: %d red-shift: %d green-shift: %d blue-shift: %d }",
		pf.BPP, pf.Depth, pf.BigEndian, pf.TrueColor, pf.RedMax, pf.GreenMax, pf.BlueMax, pf.RedShift, pf.GreenShift, pf.BlueShift)
}

func (pf PixelFormat) order() binary.ByteOrder {
	if pf.BigEndian == 1 {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

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
	ServerFramebufferUpdate  ServerMessageType = 0
	ServerSetColorMapEntries ServerMessageType = 1
	ServerBell               ServerMessageType = 2
	ServerCutText            ServerMessageType = 3
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

func (msg *FramebufferUpdateRequest) Supported(c Conn) bool {
	return true
}

// String returns string
func (msg *FramebufferUpdateRequest) String() string {
	return fmt.Sprintf("incremental: %d, x: %d, y: %d, width: %d, height: %d", msg.Inc, msg.X, msg.Y, msg.Width, msg.Height)
}

func (m *FramebufferUpdateRequest) Type() ClientMessageType { return ClientFramebufferUpdateRequest }
func (m *FramebufferUpdateRequest) Write(c Conn) error {
	buf := []byte{byte(ClientFramebufferUpdateRequest), m.Inc, byte(m.X >> 8), byte(m.X), byte(m.Y >> 8), byte(m.Y), byte(m.Width >> 8), byte(m.Width), byte(m.Height >> 8), byte(m.Height)}
	_, err := c.Write(buf)
	return err
}

// Read unmarshal message from conn
func (m *FramebufferUpdateRequest) Read(c Conn) (ClientMessage, error) {
	msg := FramebufferUpdateRequest{}
	if err := binary.Read(c, binary.BigEndian, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
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

type CutTextMessage struct {
	_      [1]byte
	Length uint32
	Text   []byte
}

// String returns string
func (m *CutTextMessage) String() string {
	return fmt.Sprintf("lenght: %d text: %s", m.Length, m.Text)
}
func (m *CutTextMessage) Type() ClientMessageType { return ClientCutText }
func (m *CutTextMessage) Write(c Conn) error {
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

type FramebufferUpdateMessage struct {
	_       [1]byte      // pad
	NumRect uint16       // number-of-rectangles
	Rects   []*Rectangle // rectangles
}

func (m *FramebufferUpdateMessage) Type() ServerMessageType { return ServerFramebufferUpdate }
func (m *FramebufferUpdateMessage) String() string {
	return fmt.Sprintf("rects %d rectangle[]: { %v }", m.NumRect, m.Rects)
}
func (msg *FramebufferUpdateMessage) Supported(c Conn) bool {
	return true
}
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

// Write marshals message to conn
func (msg *FramebufferUpdateMessage) Write(c Conn) error {
	if err := binary.Write(c, binary.BigEndian, msg.Type()); err != nil {
		return err
	}
	var pad [1]byte
	if err := binary.Write(c, binary.BigEndian, pad); err != nil {
		return err
	}
	if err := binary.Write(c, binary.BigEndian, msg.NumRect); err != nil {
		return err
	}
	for _, rect := range msg.Rects {
		if err := rect.Write(c); err != nil {
			return err
		}
	}
	return c.Flush()
}

// Color represents a single color in a color map.
type Color struct {
	pf      *PixelFormat
	cm      *ColorMap
	cmIndex uint32 // Only valid if pf.TrueColor is false.
	R, G, B uint16
}

// Read unmarshal color from conn
func (clr *Color) Read(c Conn) error {
	order := clr.pf.order()
	var pixel uint32

	switch clr.pf.BPP {
	case 8:
		var px uint8
		if err := binary.Read(c, order, &px); err != nil {
			return err
		}
		pixel = uint32(px)
	case 16:
		var px uint16
		if err := binary.Read(c, order, &px); err != nil {
			return err
		}
		pixel = uint32(px)
	case 32:
		var px uint32
		if err := binary.Read(c, order, &px); err != nil {
			return err
		}
		pixel = uint32(px)
	}

	if clr.pf.TrueColor != 0 {
		clr.R = uint16((pixel >> clr.pf.RedShift) & uint32(clr.pf.RedMax))
		clr.G = uint16((pixel >> clr.pf.GreenShift) & uint32(clr.pf.GreenMax))
		clr.B = uint16((pixel >> clr.pf.BlueShift) & uint32(clr.pf.BlueMax))
	} else {
		*clr = clr.cm[pixel]
		clr.cmIndex = pixel
	}
	return nil
}

// ColorMap represent color map
type ColorMap [256]Color

// NewColor returns a new Color object
func NewColor(pf *PixelFormat, cm *ColorMap) *Color {
	return &Color{
		pf: pf,
		cm: cm,
	}
}

type SetColorMapEntriesMessage struct {
	_          [1]byte
	FirstColor uint16
	ColorsNum  uint16
	Colors     []Color
}

func (m *SetColorMapEntriesMessage) Type() ServerMessageType { return ServerSetColorMapEntries }
func (msg *SetColorMapEntriesMessage) Supported(c Conn) bool {
	return true
}
func (msg *SetColorMapEntriesMessage) String() string {
	return fmt.Sprintf("first color: %d, numcolors: %d, colors[]: { %v }", msg.FirstColor, msg.ColorsNum, msg.Colors)
}

// Read unmrashal message from conn
func (*SetColorMapEntriesMessage) Read(c Conn) (ServerMessage, error) {
	logger.Info("Reading SetColorMapEntries message")
	msg := SetColorMapEntriesMessage{}
	var pad [1]byte
	if err := binary.Read(c, binary.BigEndian, &pad); err != nil {
		return nil, err
	}

	if err := binary.Read(c, binary.BigEndian, &msg.FirstColor); err != nil {
		return nil, err
	}

	if err := binary.Read(c, binary.BigEndian, &msg.ColorsNum); err != nil {
		return nil, err
	}

	msg.Colors = make([]Color, msg.ColorsNum)
	colorMap := c.ColorMap()

	for i := uint16(0); i < msg.ColorsNum; i++ {
		color := &msg.Colors[i]
		err := color.Read(c)
		if err != nil {
			//if err := binary.Read(c, binary.BigEndian, &color); err != nil {
			return nil, err
		}
		colorMap[msg.FirstColor+i] = *color
	}
	c.SetColorMap(colorMap)
	return &msg, nil
}

// Write marshal message to conn
func (msg *SetColorMapEntriesMessage) Write(c Conn) error {
	if err := binary.Write(c, binary.BigEndian, msg.Type()); err != nil {
		return err
	}
	var pad [1]byte
	if err := binary.Write(c, binary.BigEndian, &pad); err != nil {
		return err
	}

	if err := binary.Write(c, binary.BigEndian, msg.FirstColor); err != nil {
		return err
	}

	if msg.ColorsNum < uint16(len(msg.Colors)) {
		msg.ColorsNum = uint16(len(msg.Colors))
	}
	if err := binary.Write(c, binary.BigEndian, msg.ColorsNum); err != nil {
		return err
	}

	for i := 0; i < len(msg.Colors); i++ {
		color := msg.Colors[i]
		if err := binary.Write(c, binary.BigEndian, color); err != nil {
			return err
		}
	}

	return c.Flush()
}

type ServerBellMessage struct{}

func (*ServerBellMessage) String() string {
	return fmt.Sprintf("bell")
}
func (m *ServerBellMessage) Supported(c Conn) bool {
	return true
}
func (m *ServerBellMessage) Type() ServerMessageType            { return ServerBell }
func (m *ServerBellMessage) Read(c Conn) (ServerMessage, error) { return m, nil }

// Write marshal message to conn
func (m *ServerBellMessage) Write(c Conn) error {
	if err := binary.Write(c, binary.BigEndian, m.Type()); err != nil {
		return err
	}
	return c.Flush()
}

type ServerCutTextMessage struct {
	_      [1]byte
	Length uint32
	Text   []byte
}

func (m *ServerCutTextMessage) Type() ServerMessageType { return ServerCutText }

func (m *ServerCutTextMessage) Supported(c Conn) bool {
	return true
}

// String returns string
func (m *ServerCutTextMessage) String() string {
	return fmt.Sprintf("lenght: %d text: %s", m.Length, m.Text)
}

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

// Write marshal message to conn
func (m *ServerCutTextMessage) Write(c Conn) error {
	if err := binary.Write(c, binary.BigEndian, m.Type()); err != nil {
		return err
	}
	var pad [1]byte
	if err := binary.Write(c, binary.BigEndian, pad); err != nil {
		return err
	}

	if m.Length < uint32(len(m.Text)) {
		m.Length = uint32(len(m.Text))
	}
	if err := binary.Write(c, binary.BigEndian, m.Length); err != nil {
		return err
	}

	if err := binary.Write(c, binary.BigEndian, m.Text); err != nil {
		return err
	}
	return c.Flush()
}

type Renderer interface {
	SetTargetImage(draw.Image)
}
