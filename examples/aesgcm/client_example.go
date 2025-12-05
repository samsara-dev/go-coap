package aesgcm

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"time"

	coapNet "github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/udp"
	udpClient "github.com/plgd-dev/go-coap/v3/udp/client"
)

// DialAESGCM creates a CoAP client connection with AES-GCM encryption.
// key must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256 respectively.
func DialAESGCM(target string, key []byte, opts ...udp.Option) (*udpClient.Conn, error) {
	// Resolve target address
	raddr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	// Create UDP connection
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial UDP: %w", err)
	}

	// Wrap with AES-GCM encryption
	encryptedConn, err := NewAESGCMConn(conn, raddr, key)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create encrypted connection: %w", err)
	}

	// Wrap in CoAP's net.Conn
	coapConn := coapNet.NewConn(encryptedConn)

	// Create session using our custom AES-GCM session type
	cfg := udpClient.DefaultConfig
	for _, o := range opts {
		o.UDPClientApply(&cfg)
	}

	session := NewSession(
		context.Background(),
		coapConn,
		cfg.MaxMessageSize,
		cfg.MTU,
		true, // closeSocket
	)

	// Create client connection
	cc := udpClient.NewConnWithOpts(session, &cfg)

	return cc, nil
}

// Example usage
func ExampleClient() {
	// Generate a 32-byte key for AES-256-GCM
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}

	// Dial encrypted connection
	conn, err := DialAESGCM("localhost:5683", key)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Use the connection like any other CoAP connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := conn.Get(ctx, "/test")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Response: %v\n", resp.Code())
}

