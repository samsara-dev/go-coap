# Connectionless (Packet-Oriented) Encryption Approach

This document explains how to implement AES-GCM encryption using a **connectionless** (packet-oriented) approach that works with `udp/server/session.go`.

**Note**: This implementation demonstrates **Approach #1 (Custom Session Type)** from the [README.md](README.md#1-custom-session-type-encryption-in-session) comparison. Encryption/decryption is handled directly within the `Session` implementation rather than in a `net.Conn` wrapper.

## Key Differences from Connection-Oriented Approach

| Aspect | Connection-Oriented | Connectionless |
|--------|---------------------|----------------|
| **Session Type** | `dtls/server/session.go` style | `udp/server/session.go` style |
| **Connection Type** | `*coapNet.Conn` (stream) | `*coapNet.UDPConn` (packets) |
| **Remote Address** | Fixed per connection | Can vary per packet |
| **Encryption Layer** | In `net.Conn` wrapper | In Session's `WriteMessage`/`Run` |
| **Key Management** | One key per connection | Key per remote address |

## Architecture

```
CoAP Message
    ↓
Session.WriteMessage() → Marshal → Plain bytes
    ↓
Encrypt (using key for raddr) → Encrypted bytes
    ↓
UDPConn.WriteWithOptions() → UDP packet
    ↓
Network
```

On receive:
```
UDP packet → UDPConn.ReadWithOptions()
    ↓
Decrypt (using key for source address) → Plain bytes
    ↓
Session.Run() → Unmarshal → CoAP Message
```

## Implementation

### 1. Key Provider Interface

The `KeyProvider` interface allows different keys for different client addresses:

```go
type KeyProvider interface {
    GetKey(raddr *net.UDPAddr) ([]byte, error)
}
```

This is essential for connectionless UDP where packets can come from different addresses.

### 2. ConnectionlessEncryptedSession

The `ConnectionlessEncryptedSession` implements `udp/client.Session` and:

- **WriteMessage()**: Marshals CoAP message, encrypts it using the key for `raddr`, then writes to `UDPConn`
- **Run()**: Reads encrypted packets from `UDPConn`, decrypts using key for source address, then processes as CoAP

### 3. Key Differences from Connection-Oriented

**Connection-Oriented** (`encrypted_conn.go`):
- Encryption happens in `net.Conn` wrapper
- One connection = one remote address = one key
- Simpler, but less flexible

**Connectionless** (`connectionless_session.go`):
- Encryption happens in Session layer
- Each packet can use different key based on source/destination address
- More flexible, supports multiple clients with different keys
- Better for servers handling multiple clients

## Usage Example

```go
// Create key provider (could use different keys per address)
keyProvider := NewStaticKeyProvider(key)

// Create UDP connection
conn, _ := net.ListenUDP("udp", &net.UDPAddr{Port: 5683})
coapUDPConn, _ := coapNet.NewUDPConn("udp", conn)

// For each client, create an encrypted session
raddr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 12345}
session := NewConnectionlessEncryptedSession(
    ctx,
    doneCtx,
    coapUDPConn,
    raddr,
    maxMessageSize,
    mtu,
    false, // don't close socket (shared)
    keyProvider,
)

// Create client connection
cc := udpClient.NewConnWithOpts(session, &cfg)
```

## Advantages

1. **Multiple Clients**: Can handle multiple clients with different keys
2. **Flexible Key Management**: Key can be determined per packet/address
3. **Uses Standard UDP Session**: Works with `udp/server/session.go` pattern
4. **Connectionless**: No need to maintain connection state

## Disadvantages

1. **More Complex**: Encryption logic in session, not connection
2. **Key Lookup Overhead**: Must look up key for each packet
3. **No Stream Semantics**: Each packet encrypted independently (no stream cipher benefits)

## When to Use

- **Use Connectionless** when:
  - You need to support multiple clients with different keys
  - You want to use the standard UDP server pattern
  - You need per-address key management

- **Use Connection-Oriented** when:
  - You have a single client per connection
  - You want simpler implementation
  - You can use stream-oriented encryption

## Security Considerations

1. **Nonce Management**: Each packet uses a random nonce (no sequence numbers)
2. **Replay Protection**: Consider adding sequence numbers or timestamps
3. **Key Rotation**: The `KeyProvider` interface allows dynamic key updates
4. **Key Distribution**: In production, use proper key exchange (e.g., DTLS handshake)






