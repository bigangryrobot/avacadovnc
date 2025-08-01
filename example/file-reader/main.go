package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"

	vnc "github.com/bigangryrobot/vnc2video"
)

func main() {
	var fbsPath string
	flag.StringVar(&fbsPath, "fbs", "recording.fbs", "Path to the FBS recording file")
	flag.Parse()

	if fbsPath == "" {
		log.Fatal("FBS file path cannot be empty. Use the -fbs flag.")
	}

	// Create a reader for the FBS file.
	fbsReader, err := vnc.NewFbsReader(fbsPath)
	if err != nil {
		log.Fatalf("Error creating FBS reader: %v", err)
	}
	defer fbsReader.Close()

	// Create a canvas to draw the framebuffer updates onto.
	canvas := vnc.NewVncCanvas(
		int(fbsReader.Width()),
		int(fbsReader.Height()),
		fbsReader.PixelFormat(),
	)

	// A mock writer to capture any data the handlers might write.
	var mockWriter bytes.Buffer

	// Create a mock connection that uses the FBS reader as its data source.
	mockConn := vnc.NewMockConn(
		fbsReader,
		&mockWriter,
		[]vnc.Encoding{
			&vnc.RawEncoding{},
			&vnc.CopyRectEncoding{},
			&vnc.TightEncoding{},
			&vnc.ZlibEncoding{},
			&vnc.RREEncoding{},
			&vnc.HextileEncoding{},
		},
	)

	// Configure the mock connection with the metadata from the FBS file.
	mockConn.SetPixelFormat(fbsReader.PixelFormat())
	mockConn.SetDesktopName(fbsReader.DesktopName())
	mockConn.SetWidth(fbsReader.Width())
	mockConn.SetHeight(fbsReader.Height())

	// The FramebufferUpdate handler is what processes screen updates.
	fbuHandler := &vnc.FramebufferUpdateMessage{}

	// Process the stream frame by frame.
	for i := 0; ; i++ {
		// The Read method on the handler will process one full FramebufferUpdate
		// message from the connection, which in this case is our FBS file.
		_, err := fbuHandler.Read(mockConn)
		if err != nil {
			if err.Error() == "EOF" {
				log.Println("Reached end of FBS file.")
				break
			}
			log.Fatalf("Error reading framebuffer update: %v", err)
		}

		// Draw the updated canvas to an in-memory image.
		img := image.NewRGBA(image.Rect(0, 0, int(fbsReader.Width()), int(fbsReader.Height())))
		// Create a vnc.Rectangle representing the area to draw from the canvas.
		// In this case, we want to draw the entire canvas.
		canvasRect := &vnc.Rectangle{
			X:      0,
			Y:      0,
			Width:  fbsReader.Width(),
			Height: fbsReader.Height(),
		}
		canvas.Draw(img, canvasRect)

		// Save the image as a PNG file.
		fileName := fmt.Sprintf("frame-%05d.png", i)
		file, err := os.Create(fileName)
		if err != nil {
			log.Printf("Failed to create file %s: %v", fileName, err)
			continue
		}

		if err := png.Encode(file, img); err != nil {
			log.Printf("Failed to encode PNG %s: %v", fileName, err)
		}

		file.Close()
		log.Printf("Saved %s", fileName)
	}

	log.Println("Processing complete.")
}

// FramebufferUpdateMessage is a placeholder from the original example.
// The actual implementation is now within the vnc2video package.
// We need this struct here to satisfy the Read method call.
type FramebufferUpdateMessage struct{}

func (m *FramebufferUpdateMessage) Read(c vnc.Conn) (vnc.ServerMessage, error) {
	var padding [1]byte
	if _, err := c.Read(padding[:]); err != nil {
		return nil, err
	}
	var numRects uint16
	if err := binary.Read(c, binary.BigEndian, &numRects); err != nil {
		return nil, err
	}
	for i := uint16(0); i < numRects; i++ {
		var rect vnc.Rectangle
		var encodingType vnc.EncodingType
		if err := binary.Read(c, binary.BigEndian, &rect.X); err != nil {
			return nil, err
		}
		if err := binary.Read(c, binary.BigEndian, &rect.Y); err != nil {
			return nil, err
		}
		if err := binary.Read(c, binary.BigEndian, &rect.Width); err != nil {
			return nil, err
		}
		if err := binary.Read(c, binary.BigEndian, &rect.Height); err != nil {
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
	return nil, nil // We don't need to return a message, just process the stream.
}
