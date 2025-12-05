package main

import (
	"context"
	"log"
	"time"

	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/udp"
)

func main() {
	co, err := udp.Dial("localhost:5688")
	if err != nil {
		log.Fatalf("Error dialing: %v", err)
	}
	defer co.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Method 1: Using message.Option directly in the variadic opts parameter
	// This is the recommended approach when using client methods like Get, Post, etc.
	resp, err := co.Get(ctx, "/path",
		message.Option{
			ID:    message.URIQuery,
			Value: []byte("key1=value1"),
		},
		message.Option{
			ID:    message.URIQuery,
			Value: []byte("key2=value2"),
		},
	)
	if err != nil {
		log.Fatalf("Error sending request: %v", err)
	}
	log.Printf("Response payload: %v", resp.String())

	// Method 2: Using AddQuery when creating a request manually
	req, err := co.NewGetRequest(ctx, "/path")
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}
	defer co.ReleaseMessage(req)

	// Add query parameters using AddQuery method
	req.AddQuery("key1=value1")
	req.AddQuery("key2=value2")

	// Send the request
	resp2, err := co.Do(req)
	if err != nil {
		log.Fatalf("Error sending request: %v", err)
	}
	log.Printf("Response payload: %v", resp2.String())
}




