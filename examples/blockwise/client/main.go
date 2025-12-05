package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/net/blockwise"
	"github.com/plgd-dev/go-coap/v3/options"
	"github.com/plgd-dev/go-coap/v3/udp"
)

func main() {
	// Get server address from command line or use default
	serverAddr := "localhost:5688"
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	}

	// Get resource path from command line or use default
	path := "/large-resource"
	if len(os.Args) > 2 {
		path = os.Args[2]
	}

	// Create a UDP client with blockwise enabled
	// Blockwise is enabled by default, but we can explicitly configure it:
	// - SZX1024: block size of 1024 bytes
	// - 3 second timeout for blockwise transfers
	co, err := udp.Dial(serverAddr,
		options.WithBlockwise(true, blockwise.SZX1024, 3*time.Second),
	)
	if err != nil {
		log.Fatalf("Error dialing: %v", err)
	}
	defer co.Close()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Perform a GET request
	// The blockwise transfer will be handled automatically if the response
	// is larger than the block size
	fmt.Printf("Performing GET request to %s%s\n", serverAddr, path)
	resp, err := co.Get(ctx, path)
	if err != nil {
		log.Fatalf("Error sending request: %v", err)
	}

	// Read the response body
	// For blockwise transfers, the entire response is automatically
	// assembled from multiple blocks
	body, err := io.ReadAll(resp.Body())
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	// Print response information
	fmt.Printf("\nResponse Code: %v\n", resp.Code())
	fmt.Printf("Response Size: %d bytes\n", len(body))
	fmt.Printf("Response Body (first 200 chars): %s\n", string(body[:min(200, len(body))]))
	if len(body) > 200 {
		fmt.Printf("... (truncated, total %d bytes)\n", len(body))
	}

	// Check if blockwise was used by checking for Block2 option
	// Note: Block2 option is typically removed from the final assembled message,
	// but the blockwise transfer happened automatically if the response was large
	fmt.Printf("\nBlockwise transfer was handled automatically if response exceeded block size\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

