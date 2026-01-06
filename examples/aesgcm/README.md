# Custom Encryption with AES-GCM

This example demonstrates how to hook in custom encryption (AES-GCM) into the CoAP library, similar to how DTLS is integrated.

## Architecture

The key insight is that the CoAP library uses the `net.Conn` interface for connections. Any type that implements `net.Conn` can be wrapped with `coapNet.NewConn()` and used seamlessly with the CoAP stack.

### Flow

```
CoAP Message
    ↓
MarshalWithEncoder() → Plain CoAP bytes
    ↓
WriteWithContext() → net.Conn wrapper
    ↓
AESGCMConn.Write() → AES-GCM encryption
    ↓
UDP socket → Encrypted packets on wire
```

## Implementation

### 1. Create a Custom Connection Type

The `AESGCMConn` type implements `net.Conn` and wraps a UDP connection:

- **Write()**: Encrypts plain CoAP bytes with AES-GCM before writing to UDP
- **Read()**: Reads encrypted packets from UDP and decrypts them

### 2. Integration Points

**For Clients:**
```go
// Create UDP connection
conn, _ := net.DialUDP("udp", nil, raddr)

// Wrap with encryption
encryptedConn, _ := NewAESGCMConn(conn, raddr, key)

// Wrap in CoAP's net.Conn
coapConn := coapNet.NewConn(encryptedConn)

// Create session and client connection
session := NewSession(...)  // Custom session type
cc := udpClient.NewConnWithOpts(session, &cfg)
```

**For Servers:**
Similar pattern, but you need to handle multiple clients (connection management).

## Key Points

1. **Transparent Encryption**: The CoAP library doesn't know about encryption - it just writes plain bytes to a connection that happens to encrypt them.

2. **Same Pattern as DTLS**: This follows the exact same pattern as DTLS integration:
   - DTLS wraps `*pion/dtls.Conn` (which implements `net.Conn`)
   - AES-GCM wraps a custom connection (which implements `net.Conn`)

3. **Session Type**: You need to implement the `udp/client.Session` interface. This example includes a custom `Session` type (`session.go`) that handles the CoAP message marshaling and reading loop. You could also reuse `dtlsServer.NewSession()` since it works with any `*coapNet.Conn`, but having your own session type is cleaner and more explicit.

## Considerations

1. **Key Management**: In production, you'd need secure key exchange/management (unlike this example which uses a shared key).

2. **Connection State**: For UDP, you may want to maintain connection state per client address.

3. **Nonce Management**: The example uses random nonces. For better security, consider:
   - Sequence numbers
   - Replay protection
   - Connection-specific nonces

4. **Performance**: Consider using a connection pool and reusing encrypted connections.

## Alternative Approaches

If you need more control or different encryption schemes, here are three alternative approaches to consider:

### 1. Custom Session Type (Encryption in Session)

**Description**: Handle encryption/decryption directly within the `Session` implementation, rather than in a `net.Conn` wrapper.

**How it works**:
- The `Session.WriteMessage()` method marshals the CoAP message, then encrypts the bytes before writing to the underlying connection
- The `Session.Run()` method reads encrypted bytes from the connection, decrypts them, then processes as CoAP messages
- The underlying connection remains a plain UDP connection

**Example flow**:
```
CoAP Message → Session.WriteMessage() → Marshal → Encrypt → UDP connection
UDP connection → Session.Run() → Decrypt → Unmarshal → CoAP Message
```

**Pros**:
- ✅ Full control over encryption at the CoAP message boundary
- ✅ Can implement per-message encryption schemes (e.g., different keys per message)
- ✅ Easier to implement connectionless encryption (different keys per remote address)
- ✅ Encryption logic is explicit and visible in the session code
- ✅ Can easily add encryption metadata or headers per message

**Cons**:
- ❌ Encryption logic is tightly coupled to the session implementation
- ❌ Less reusable - encryption can't be easily swapped out
- ❌ Session must handle both CoAP protocol and encryption concerns
- ❌ More code duplication if you need multiple session types with different encryption

**Best for**: Connectionless UDP scenarios, per-message encryption schemes, or when you need encryption logic that's aware of CoAP message boundaries.

**Example**: See `connectionless_session.go` and `CONNECTIONLESS.md` for a complete implementation of this approach.

---

### 2. Middleware Pattern (Connection Wrapper)

**Description**: Create a reusable encryption middleware that wraps any `net.Conn` before passing it to CoAP. This is the approach used in the current example.

**How it works**:
- Create a `net.Conn` wrapper (like `AESGCMConn`) that encrypts on `Write()` and decrypts on `Read()`
- Wrap the raw connection with encryption middleware
- Pass the encrypted connection to `coapNet.NewConn()` and use with any session type

**Example flow**:
```
CoAP Message → Session → coapNet.Conn → EncryptedConn.Write() → Encrypt → UDP
UDP → EncryptedConn.Read() → Decrypt → coapNet.Conn → Session → CoAP Message
```

**Pros**:
- ✅ **Separation of concerns**: Encryption is decoupled from CoAP protocol logic
- ✅ **Reusable**: Same encryption wrapper works with any session type (DTLS-style, UDP-style, etc.)
- ✅ **Transparent**: CoAP library doesn't know about encryption - it just sees a `net.Conn`
- ✅ **Composable**: Can stack multiple middleware layers (encryption, compression, etc.)
- ✅ **Matches DTLS pattern**: Follows the same approach as the library's DTLS integration
- ✅ **Easy to test**: Can test encryption independently of CoAP

**Cons**:
- ❌ Less control over per-message encryption (encryption happens at byte stream level)
- ❌ For connectionless UDP, you need connection management (one encrypted connection per remote address)
- ❌ Slightly more complex setup (wrapping layers)

**Best for**: Connection-oriented scenarios, reusable encryption schemes, or when you want encryption to be transparent to the CoAP layer. This is the recommended approach for most use cases.

---

### 3. Transport Layer (Custom Transport Implementation)

**Description**: Implement a custom transport layer that sits between the network socket and the CoAP stack, handling encryption before CoAP sees any data.

**How it works**:
- Create a custom transport implementation that wraps the network layer
- The transport handles encryption/decryption of all data
- CoAP operates on top of this encrypted transport, treating it as a regular transport

**Example flow**:
```
CoAP Message → CoAP Stack → Custom Transport → Encrypt → Network Socket
Network Socket → Custom Transport → Decrypt → CoAP Stack → CoAP Message
```

**Pros**:
- ✅ **Lowest-level control**: Encryption happens at the transport boundary
- ✅ **Protocol-agnostic**: Could theoretically work with any protocol, not just CoAP
- ✅ **Centralized**: All encryption logic in one place
- ✅ **Library integration**: Could be integrated into the CoAP library itself

**Cons**:
- ❌ **Most complex**: Requires implementing transport interfaces, which may not be public
- ❌ **Library changes**: May require modifications to the CoAP library itself
- ❌ **Less flexible**: Harder to swap encryption schemes or disable encryption
- ❌ **Overkill**: Usually unnecessary unless you're building a new transport from scratch

**Best for**: Building a new transport protocol, library-level integration, or when you need encryption at the absolute lowest level. Generally not recommended unless you're extending the library itself.

---

## Comparison Summary

| Aspect | Custom Session | Middleware Pattern | Transport Layer |
|--------|---------------|-------------------|-----------------|
| **Complexity** | Medium | Low | High |
| **Reusability** | Low | High | Medium |
| **Separation of Concerns** | Medium | High | High |
| **Per-Message Control** | High | Low | Low |
| **Connectionless Support** | Easy | Requires management | Depends on implementation |
| **Library Integration** | No changes needed | No changes needed | May need changes |
| **Recommended For** | Connectionless UDP, per-message schemes | Most use cases | Library extensions |

---

### 4. Payload-Only Encryption with Post-Serialization MAC

**Description**: Encrypt only the CoAP payload before passing it to the library, then append a MAC to the serialized CoAP packet after the library serializes it but before transmission.

**How it works**:
- Encrypt the payload before setting it in the CoAP message
- CoAP library serializes the message (header + encrypted payload)
- Append MAC to the serialized bytes before transmission
- On receive: Verify MAC, then unmarshal, then decrypt payload

**Example flow**:
```
Plain Payload → Encrypt → CoAP Message (with encrypted payload)
    ↓
CoAP Library serializes → [Header][Encrypted Payload]
    ↓
Append MAC → [Header][Encrypted Payload][MAC] → UDP
    ↓
On receive: Verify MAC → Unmarshal → Decrypt Payload
```

**Pros**:
- ✅ **De-duplication preserved**: CoAP headers (MessageID, Token) remain visible for de-duplication
- ✅ **Selective encryption**: Only payload is encrypted, headers remain plain
- ✅ **CoAP-aware**: Works with CoAP's message structure

**Cons**:
- ❌ **MAC placement complexity**: You need to know where the CoAP message ends to extract/verify the MAC. This requires either:
  - Parsing the CoAP format to find message boundaries (complex, error-prone)
  - Using fixed-size MAC and extracting last N bytes (works but breaks round-trip integrity)
- ❌ **Round-trip integrity broken**: CoAP expects to serialize and unmarshal the exact same bytes. Appending MAC means the bytes you send ≠ bytes you receive (after MAC verification)
- ❌ **Session-level interception required**: You must intercept at the session's `WriteMessage()` and `Run()` methods, similar to Approach #1, but with additional complexity
- ❌ **Response cache issues**: The response cache stores message objects (not bytes), so this is actually fine, but you still need custom session logic
- ❌ **Non-standard format**: You're creating a modified CoAP format that other implementations won't understand
- ❌ **Two-stage processing overhead**: Serialize → Add MAC on send; Verify MAC → Unmarshal on receive adds complexity

**De-duplication Analysis**:
- ✅ **Will work**: CoAP de-duplicates based on MessageID (in the header), which remains unencrypted
- ✅ **Response cache works**: The cache stores message objects, not serialized bytes, so it's unaffected
- ⚠️ **But**: You still need custom session logic to handle MAC verification before unmarshaling

**Is this a bad idea?**
It's not necessarily "bad," but it's **more complex than necessary** and introduces several challenges:

1. **MAC extraction**: The hardest part is knowing where the CoAP message ends. Options:
   - **Fixed-size MAC**: Extract last N bytes (e.g., 16 bytes for AES-GCM tag). Works but requires custom unmarshaling logic.
   - **Parse CoAP format**: Find the payload separator (0xFF) and calculate message length. Complex and error-prone.

2. **Better alternatives**:
   - **Use Approach #1 (Custom Session)**: Encrypt the entire serialized message, including headers. Simpler, and you can still do de-duplication by caching MessageIDs separately.
   - **Use Approach #2 (Middleware)**: Encrypt everything transparently. De-duplication still works because it happens before encryption/decryption.
   - **Hybrid**: Encrypt payload in session, but use authenticated encryption (AES-GCM) which includes the MAC in the ciphertext. No need to append MAC separately.

**Recommended Implementation** (if you pursue this approach):
```go
// In Session.WriteMessage():
1. Encrypt payload before setting in message
2. Marshal message to get serialized bytes
3. Calculate MAC over serialized bytes (or just headers + encrypted payload)
4. Append MAC to serialized bytes
5. Write to connection

// In Session.Run():
1. Read packet from connection
2. Extract last N bytes as MAC
3. Verify MAC on remaining bytes
4. Unmarshal remaining bytes (without MAC)
5. Decrypt payload
6. Process message
```

**Best for**: Scenarios where you need payload-only encryption but want to preserve header visibility for routing/de-duplication. However, Approach #1 or #2 are usually simpler and more maintainable.

---

## Comparison Summary

| Aspect | Custom Session | Middleware Pattern | Transport Layer | Payload-Only + MAC |
|--------|---------------|-------------------|-----------------|-------------------|
| **Complexity** | Medium | Low | High | High |
| **Reusability** | Low | High | Medium | Low |
| **Separation of Concerns** | Medium | High | High | Low |
| **Per-Message Control** | High | Low | Low | Medium |
| **Connectionless Support** | Easy | Requires management | Depends on implementation | Easy |
| **Library Integration** | No changes needed | No changes needed | May need changes | No changes needed |
| **De-duplication** | Works | Works | Works | Works (headers visible) |
| **Header Visibility** | Encrypted | Encrypted | Encrypted | Plain |
| **Recommended For** | Connectionless UDP, per-message schemes | Most use cases | Library extensions | Special cases requiring header visibility |

## Current Implementation

This example uses the **Middleware Pattern** (Approach #2), which provides a good balance of simplicity, reusability, and transparency. The `AESGCMConn` type wraps a UDP connection, and the `Session` type uses it transparently through `coapNet.Conn`.

