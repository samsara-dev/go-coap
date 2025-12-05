package aesgcm

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"

	coapNet "github.com/plgd-dev/go-coap/v3/net"
	udpClient "github.com/plgd-dev/go-coap/v3/udp/client"
	"github.com/plgd-dev/go-coap/v3/net/responsewriter"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
)

// ListenAndServeAESGCM creates a CoAP server with AES-GCM encryption.
// key must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256 respectively.
func ListenAndServeAESGCM(network, addr string, key []byte, handler udpClient.HandlerFunc) error {
	// Listen on UDP
	udpConn, err := net.ListenUDP(network, nil)
	if err != nil {
		return fmt.Errorf("failed to listen UDP: %w", err)
	}
	defer udpConn.Close()

	// For server, we need to handle multiple clients
	// This is a simplified example - in production you'd need connection management
	for {
		// Read from UDP to get client address
		buf := make([]byte, 4096)
		n, raddr, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		// For each client, create an encrypted connection
		// In practice, you'd want to cache these connections
		go func(raddr *net.UDPAddr) {
			encryptedConn, err := NewAESGCMConn(udpConn, raddr, key)
			if err != nil {
				return
			}

			coapConn := coapNet.NewConn(encryptedConn)

			cfg := udpClient.DefaultConfig
			cfg.Handler = handler

			session := NewSession(
				context.Background(),
				coapConn,
				cfg.MaxMessageSize,
				cfg.MTU,
				false, // don't close socket, we're sharing it
			)

			cc := udpClient.NewConnWithOpts(session, &cfg)
			defer cc.Close()

			// Process connection
			if err := cc.Run(); err != nil {
				fmt.Printf("Connection error: %v\n", err)
			}
		}(raddr)
	}
}

// Example usage
func ExampleServer() {
	// Generate a 32-byte key for AES-256-GCM
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}

	handler := func(w *responsewriter.ResponseWriter[*udpClient.Conn], r *pool.Message) {
		// Handle CoAP request
		fmt.Printf("Received: %v\n", r.Code())
		// Set response using the response writer
		_ = w.SetResponse(codes.Content, message.TextPlain, nil)
	}

	if err := ListenAndServeAESGCM("udp", ":5683", key, handler); err != nil {
		panic(err)
	}
}

