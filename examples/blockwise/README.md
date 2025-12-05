# Blockwise GET Example

This example demonstrates how to perform a blockwise GET request using the go-coap library.

## What is Blockwise Transfer?

Blockwise transfer (RFC 7959) allows CoAP to transfer large payloads by splitting them into smaller blocks. This is useful when:
- The payload is larger than the maximum message size
- The network has MTU limitations
- You want to transfer large resources efficiently

## How It Works

The go-coap library handles blockwise transfers automatically when:
1. Blockwise is enabled (enabled by default)
2. The response payload exceeds the configured block size
3. The server supports blockwise transfers

For GET requests, the client automatically:
- Sends the initial GET request
- Receives the first block with Block2 option indicating more blocks
- Automatically requests subsequent blocks
- Assembles all blocks into a complete response

## Running the Example

### 1. Start the Server

```bash
go run examples/blockwise/server/main.go
```

The server will:
- Listen on `:5688`
- Serve `/large-resource` (5000 bytes - requires blockwise)
- Serve `/small-resource` (small payload - no blockwise needed)

### 2. Run the Client

```bash
# Get the large resource (will use blockwise transfer)
go run examples/blockwise/client/main.go localhost:5688 /large-resource

# Get the small resource (no blockwise needed)
go run examples/blockwise/client/main.go localhost:5688 /small-resource
```

## Configuration

You can configure blockwise transfer using the `WithBlockwise` option:

```go
co, err := udp.Dial(serverAddr,
    options.WithBlockwise(
        true,                    // enable blockwise
        blockwise.SZX1024,       // block size (1024 bytes)
        3*time.Second,           // transfer timeout
    ),
)
```

### Block Sizes (SZX)

- `SZX16` - 16 bytes
- `SZX32` - 32 bytes
- `SZX64` - 64 bytes
- `SZX128` - 128 bytes
- `SZX256` - 256 bytes
- `SZX512` - 512 bytes
- `SZX1024` - 1024 bytes (default)
- `SZXBERT` - 1024 bytes (BERT mode)

## Notes

- Blockwise is enabled by default, so you don't need to explicitly enable it
- The library automatically handles all blockwise protocol details
- You just call `Get()` as normal - blockwise happens transparently
- The response body contains the complete assembled payload






