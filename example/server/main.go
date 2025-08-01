package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bigangryrobot/avacadovnc"
	"github.com/bigangryrobot/avacadovnc/logger"

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
	cfg := &avacadovnc.ServerConfig{
		SecurityHandlers: []avacadovnc.SecurityHandler{
			&avacadovnc.SecurityNone{},
		},
		PixelFormat: avacadovnc.DefaultPixelFormat,
		DesktopName: "avacadovnc-server",
		Width:       1024,
		Height:      768,
		// Define the handlers for the server handshake process.
		Handlers: []avacadovnc.Handler{
			&avacadovnc.DefaultServerVersionHandler{},
			&avacadovnc.DefaultServerSecurityHandler{},
			&avacadovnc.DefaultServerClientInitHandler{},
			&avacadovnc.DefaultServerServerInitHandler{},
			// A message handler would go here to manage the session.
		},
	}

	server, err := avacadovnc.NewServer(cfg)
	if err != nil {
		logger.Fatalf("failed to create server: %v", err)
	}

	// Start the server. This will block until the server is stopped.
	if err := server.Start(*addr); err != nil {
		logger.Fatalf("VNC server failed: %v", err)
	}
}
