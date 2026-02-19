package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"

	"mini-kv/internal"
)

func generateNodeID() string {
	var b [8]byte
	for i := range b {
		b[i] = byte(i)
	}
	return fmt.Sprintf("node-%x", b)
}

func main() {
	internal.ParseFlags()

	if internal.NodeID == "" {
		internal.NodeID = generateNodeID()
	}

	addr := internal.GetAddress()
	gossipAddr := internal.GetGossipAddress()

	server := internal.NewServer(addr, gossipAddr)

	if internal.TLSCertFile != "" && internal.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(internal.TLSCertFile, internal.TLSKeyFile)
		if err != nil {
			log.Fatalf("Failed to load TLS certificate: %v", err)
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		server.SetTLSConfig(tlsConfig)
	}

	internal.SetLogger(log.New(os.Stdout, "", 0))

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
