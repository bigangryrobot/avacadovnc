package avacadovnc

// DesktopSizeEncoding implements the DesktopSize pseudo-encoding, which is used
// by the server to inform the client that the framebuffer has been resized.
type DesktopSizeEncoding struct{}

// Type returns the encoding type identifier.
func (e *DesktopSizeEncoding) Type() EncodingType {
	return EncDesktopSize
}

// Read handles the resize event. The new dimensions are in the rectangle header.
func (e *DesktopSizeEncoding) Read(c Conn, rect *Rectangle) error {
	// The new width and height are in the rectangle's header.
	c.SetWidth(rect.Width)
	c.SetHeight(rect.Height)

	// A real client would now resize its local framebuffer/canvas.
	if clientConn, ok := c.(*ClientConn); ok && clientConn.Canvas != nil {
		// This part is complex: it requires re-allocating the canvas image
		// and potentially re-requesting a full screen update.
		// For now, we just log it.
	}

	return nil
}

// Reset does nothing as this encoding is stateless.
func (e *DesktopSizeEncoding) Reset() {}
