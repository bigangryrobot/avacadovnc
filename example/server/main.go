package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bigangryrobot/vnc2video"
	"github.com/bigangryrobot/vnc2video/logger"
)

func main() {
	addr := flag.String("addr", ":5900", "Listen address for VNC server")
	flag.Parse()

	if *addr == "" {
		fmt.Println("Error: Listen address is required.")
		flag.Usage()
		os.Exit(1)
	}

	//logger.SetLevel(logger.InfoLevel)

	// Configure the VNC server.
	cfg := &vnc2video.ServerConfig{
		SecurityHandlers: []vnc2video.SecurityHandler{
			&vnc2video.SecurityNone{},
		},
		PixelFormat: vnc2video.DefaultPixelFormat,
		DesktopName: "vnc2video-server",
		Width:       1024,
		Height:      768,
		// Define the handlers for the server handshake process.
		Handlers: []vnc2video.Handler{
			&vnc2video.DefaultServerVersionHandler{},
			&vnc2video.DefaultServerSecurityHandler{},
			&vnc2video.DefaultServerClientInitHandler{},
			&vnc2video.DefaultServerServerInitHandler{},
			// A message handler would go here to manage the session.
		},
	}

	server, err := vnc2video.NewServer(cfg)
	if err != nil {
		logger.Fatalf("failed to create server: %v", err)
	}

	// Start the server. This will block until the server is stopped.
	if err := server.Start(*addr); err != nil {
		logger.Fatalf("VNC server failed: %v", err)
	}
}
