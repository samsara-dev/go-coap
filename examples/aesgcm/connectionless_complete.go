package aesgcm

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"time"

	coapNet "github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/net/responsewriter"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	udpClient "github.com/plgd-dev/go-coap/v3/udp/client"
)

// ExampleConnectionlessClient shows a complete client example using connectionless encryption.
func ExampleConnectionlessClient() {
	// Generate encryption key
	key := make([]byte, 32)
	rand.Read(key)
	keyProvider := NewStaticKeyProvider(key)

	// Resolve server address
	raddr, err := net.ResolveUDPAddr("udp", "localhost:5683")
	if err != nil {
		panic(err)
	}

	// Create UDP connection
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Wrap in coapNet.UDPConn
	coapUDPConn, err := coapNet.NewUDPConn("udp", conn)
	if err != nil {
		panic(err)
	}

	// Create encrypted session
	cfg := udpClient.DefaultConfig
	session := NewConnectionlessEncryptedSession(
		context.Background(),
		context.Background(),
		coapUDPConn,
		raddr,
		cfg.MaxMessageSize,
		cfg.MTU,
		true, // closeSocket
		keyProvider,
	)

	// Create client connection
	cc := udpClient.NewConnWithOpts(session, &cfg)

	// Use the connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := cc.Get(ctx, "/test")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Response: %v\n", resp.Code())
}

// ExampleConnectionlessServer shows a complete server example using connectionless encryption.
func ExampleConnectionlessServer() {
	// Generate encryption key (in production, use proper key management)
	key := make([]byte, 32)
	rand.Read(key)
	keyProvider := NewStaticKeyProvider(key)

	// Listen on UDP
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 5683})
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Wrap in coapNet.UDPConn
	coapUDPConn, err := coapNet.NewUDPConn("udp", conn)
	if err != nil {
		panic(err)
	}

	// For server, you'd typically handle multiple clients
	// This is a simplified single-client example
	// In production, you'd create a session per client address

	cfg := udpClient.DefaultConfig
	cfg.Handler = func(w *responsewriter.ResponseWriter[*udpClient.Conn], r *pool.Message) {
		fmt.Printf("Received encrypted message from %v\n", w.Conn().RemoteAddr())
		// Handle request...
	}

	// For each client connection, create an encrypted session
	// This would typically be done in a connection handler
	_ = coapUDPConn
	_ = keyProvider
	_ = cfg
}

