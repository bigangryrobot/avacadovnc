package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	vnc "github.com/bigangryrobot/avacadovnc"
	"github.com/bigangryrobot/avacadovnc/logger"
)

func main() {
	// --- Command-line flags ---
	var host, password string
	var port int
	flag.StringVar(&host, "host", "127.0.0.1", "VNC server host")
	flag.IntVar(&port, "port", 5900, "VNC server port")
	flag.StringVar(&password, "password", "", "VNC server password")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", host, port)
	logger.Infof("Connecting to VNC server at %s", addr)

	// --- VNC Client Configuration ---
	// The message channels are used to communicate between the main application
	// logic and the connection's message handling goroutines.
	clientCh := make(chan vnc.ClientMessage, 10)
	serverCh := make(chan vnc.ServerMessage, 10)

	cfg := &vnc.ClientConfig{
		SecurityHandlers: []vnc.SecurityHandler{
			&vnc.SecurityNone{},
		},
		Encodings: []vnc.Encoding{
			&vnc.RawEncoding{},
			&vnc.CopyRectEncoding{},
			&vnc.TightEncoding{},
			&vnc.ZlibEncoding{},
			&vnc.RREEncoding{},
			&vnc.HextileEncoding{},
			// Pseudo-encodings handle events like desktop resizes and cursor updates.
			// The library uses these to manage the canvas state automatically.
			&vnc.DesktopSizeEncoding{},
			&vnc.CursorEncoding{},
		},
		PixelFormat:     vnc.DefaultPixelFormat,
		ClientMessageCh: clientCh,
		ServerMessageCh: serverCh,
		Messages: []vnc.ServerMessage{
			&vnc.FramebufferUpdateMessage{},
			&vnc.SetColourMapEntriesMessage{},
			&vnc.ServerBellMessage{},
			&vnc.ServerCutTextMessage{},
		},
		DrawCursor: true, // Tell the canvas to render the mouse pointer.
	}

	// --- Connection ---
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to connect to VNC server: %v", err)
	}

	// The Connect function performs the VNC handshake.
	clientConn, err := vnc.Connect(context.Background(), conn, cfg)
	if err != nil {
		log.Fatalf("VNC handshake failed: %v", err)
	}
	defer clientConn.Close()

	logger.Info("VNC connection established successfully.")

	// --- Canvas and Frame Processing ---
	// The VncCanvas holds the state of the remote framebuffer.
	// It's initialized with the dimensions received during the handshake.
	canvas := vnc.NewVncCanvas(
		int(clientConn.Width()),
		int(clientConn.Height()),
		clientConn.PixelFormat(),
	)
	// The client connection needs a reference to the canvas to draw updates.
	clientConn.Canvas = canvas

	// --- Main Event Loop ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown on interrupt signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Interrupt signal received, shutting down.")
		cancel()
	}()

	// Start by requesting the first full framebuffer update.
	// Subsequent requests will be for incremental updates.
	clientCh <- &vnc.FramebufferUpdateRequest{
		Inc:    0, // 0 = full update, 1 = incremental
		X:      0,
		Y:      0,
		Width:  clientConn.Width(),
		Height: clientConn.Height(),
	}

	frameCount := 0
	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-serverCh:
			if !ok {
				logger.Info("Server channel closed, exiting.")
				return
			}

			switch msg.(type) {
			case *vnc.FramebufferUpdateMessage:
				saveFrame(canvas, frameCount)
				frameCount++

				// Wait a moment before requesting the next frame to avoid flooding the server.
				time.Sleep(100 * time.Millisecond)

				// Request the next incremental update.
				clientCh <- &vnc.FramebufferUpdateRequest{
					Inc:    1,
					X:      0,
					Y:      0,
					Width:  clientConn.Width(),
					Height: clientConn.Height(),
				}
			}
		}
	}
}

// saveFrame saves the current state of the VncCanvas to a PNG file.
func saveFrame(canvas *vnc.VncCanvas, frameIndex int) {
	bounds := image.Rect(0, 0, canvas.Width(), canvas.Height())
	img := image.NewRGBA(bounds)

	// The canvas's Draw method renders its current state onto our image.
	canvas.Draw(img, &vnc.Rectangle{
		X: 0, Y: 0, Width: uint16(bounds.Dx()), Height: uint16(bounds.Dy()),
	})

	fileName := fmt.Sprintf("frame-%05d.png", frameIndex)
	file, err := os.Create(fileName)
	if err != nil {
		logger.Errorf("Failed to create file %s: %v", fileName, err)
		return
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		logger.Errorf("Failed to encode PNG %s: %v", fileName, err)
		return
	}

	logger.Infof("Saved frame: %s", fileName)
}
