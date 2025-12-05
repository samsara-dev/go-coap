package main

import (
	"bytes"
	"log"

	coap "github.com/plgd-dev/go-coap/v3"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
)

func handleLargeResource(w mux.ResponseWriter, _ *mux.Message) {
	// Create a large payload (larger than typical block size of 1024 bytes)
	// This will trigger blockwise transfer
	largeData := make([]byte, 5000) // 5000 bytes - will require multiple blocks
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err := w.SetResponse(codes.Content, message.TextPlain, bytes.NewReader(largeData))
	if err != nil {
		log.Printf("cannot set response: %v", err)
	}
}

func handleSmallResource(w mux.ResponseWriter, _ *mux.Message) {
	// Small resource that won't require blockwise transfer
	err := w.SetResponse(codes.Content, message.TextPlain, bytes.NewReader([]byte("Hello, World!")))
	if err != nil {
		log.Printf("cannot set response: %v", err)
	}
}

func main() {
	r := mux.NewRouter()
	
	// Handler for large resource that will use blockwise transfer
	r.Handle("/large-resource", mux.HandlerFunc(handleLargeResource))
	
	// Handler for small resource that won't use blockwise transfer
	r.Handle("/small-resource", mux.HandlerFunc(handleSmallResource))

	log.Println("Starting CoAP server on :5688")
	log.Println("Try GET /large-resource to see blockwise transfer in action")
	log.Fatal(coap.ListenAndServe("udp", ":5688", r))
}






