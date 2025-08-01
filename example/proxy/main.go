package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/bigangryrobot/vnc2video/logger"

)

func main() {
	listenAddr := flag.String("listen", ":5901", "Listen address for the proxy")
	remoteAddr := flag.String("remote", "127.0.0.1:5900", "Remote VNC server address")
	flag.Parse()

	if *listenAddr == "" || *remoteAddr == "" {
		fmt.Println("Error: Both listen and remote addresses are required.")
		flag.Usage()
		os.Exit(1)
	}

	//logger.SetLevel(logger.InfoLevel)
	logger.Infof("starting VNC proxy: listening on %s, connecting to %s", *listenAddr, *remoteAddr)

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		logger.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			logger.Errorf("failed to accept client connection: %v", err)
			continue
		}
		go handleProxyConnection(clientConn, *remoteAddr)
	}
}

func handleProxyConnection(client net.Conn, remoteAddr string) {
	defer client.Close()
	logger.Infof("accepted connection from %s", client.RemoteAddr())

	server, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		logger.Errorf("failed to connect to remote server %s: %v", remoteAddr, err)
		return
	}
	defer server.Close()
	logger.Infof("connected to remote server %s", server.RemoteAddr())

	var wg sync.WaitGroup
	wg.Add(2)

	// Copy data from client to server
	go func() {
		defer wg.Done()
		if _, err := io.Copy(server, client); err != nil {
			logger.Warnf("error copying from client to server: %v", err)
		}
		server.Close() // Close the other connection when one side closes.
	}()

	// Copy data from server to client
	go func() {
		defer wg.Done()
		if _, err := io.Copy(client, server); err != nil {
			logger.Warnf("error copying from server to client: %v", err)
		}
		client.Close() // Close the other connection when one side closes.
	}()

	wg.Wait()
	logger.Infof("proxy connection for %s closed", client.RemoteAddr())
}
