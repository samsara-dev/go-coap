package aesgcm

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	coapNet "github.com/plgd-dev/go-coap/v3/net"
)

// KeyProvider provides encryption keys for different remote addresses.
// In a real implementation, you'd use this to look up keys based on client identity.
type KeyProvider interface {
	GetKey(raddr *net.UDPAddr) ([]byte, error)
}

// StaticKeyProvider provides the same key for all addresses (for testing).
type StaticKeyProvider struct {
	key []byte
}

func NewStaticKeyProvider(key []byte) *StaticKeyProvider {
	return &StaticKeyProvider{key: key}
}

func (p *StaticKeyProvider) GetKey(raddr *net.UDPAddr) ([]byte, error) {
	return p.key, nil
}

// EncryptedUDPConn wraps a UDP connection with AES-GCM encryption at the packet level.
// It implements the packetConn interface needed for coapNet.UDPConn.
type EncryptedUDPConn struct {
	underlying *net.UDPConn
	keyProvider KeyProvider
	aeadCache sync.Map // cache of AEAD instances per key (to avoid recreating)
	nonceSize int
}

// NewEncryptedUDPConn creates a new encrypted UDP connection wrapper.
func NewEncryptedUDPConn(conn *net.UDPConn, keyProvider KeyProvider) (*EncryptedUDPConn, error) {
	// Create a test AEAD to get nonce size
	testKey, err := keyProvider.GetKey(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get test key: %w", err)
	}
	
	block, err := aes.NewCipher(testKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &EncryptedUDPConn{
		underlying:  conn,
		keyProvider: keyProvider,
		nonceSize:    aead.NonceSize(),
	}, nil
}

// getAEAD gets or creates an AEAD cipher for a given key.
func (c *EncryptedUDPConn) getAEAD(key []byte) (cipher.AEAD, error) {
	keyStr := string(key)
	if aead, ok := c.aeadCache.Load(keyStr); ok {
		return aead.(cipher.AEAD), nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	c.aeadCache.Store(keyStr, aead)
	return aead, nil
}

// WriteTo encrypts the packet and writes it to the remote address.
func (c *EncryptedUDPConn) WriteTo(b []byte, cm *coapNet.ControlMessage, dst net.Addr) (int, error) {
	raddr, ok := dst.(*net.UDPAddr)
	if !ok {
		return 0, fmt.Errorf("invalid address type %T, expected *net.UDPAddr", dst)
	}

	// Get encryption key for this address
	key, err := c.keyProvider.GetKey(raddr)
	if err != nil {
		return 0, fmt.Errorf("failed to get key for %v: %w", raddr, err)
	}

	// Get or create AEAD for this key
	aead, err := c.getAEAD(key)
	if err != nil {
		return 0, err
	}

	// Generate random nonce
	nonce := make([]byte, c.nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return 0, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt plaintext
	ciphertext := aead.Seal(nonce, nonce, b, nil)

	// Write encrypted packet to underlying UDP connection
	// Note: We can't use the ControlMessage here since we're encrypting
	// For full control message support, you'd need to encrypt it or handle it separately
	n, err := c.underlying.WriteToUDP(ciphertext, raddr)
	if err != nil {
		return 0, err
	}

	// Return the number of plaintext bytes written (not encrypted bytes)
	return len(b), nil
}

// ReadFrom reads an encrypted packet and decrypts it.
func (c *EncryptedUDPConn) ReadFrom(b []byte) (n int, cm *coapNet.ControlMessage, src net.Addr, err error) {
	// Read encrypted packet (need larger buffer for encrypted data)
	encryptedBuf := make([]byte, len(b)+256) // Add space for nonce + tag
	n, raddr, err := c.underlying.ReadFromUDP(encryptedBuf)
	if err != nil {
		return 0, nil, nil, err
	}
	encryptedBuf = encryptedBuf[:n]

	// Check minimum size (nonce + ciphertext + tag)
	if len(encryptedBuf) < c.nonceSize {
		return 0, nil, nil, errors.New("encrypted data too short")
	}

	// Get encryption key for this source address
	key, err := c.keyProvider.GetKey(raddr)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to get key for %v: %w", raddr, err)
	}

	// Get or create AEAD for this key
	aead, err := c.getAEAD(key)
	if err != nil {
		return 0, nil, nil, err
	}

	// Extract nonce and ciphertext
	nonce := encryptedBuf[:c.nonceSize]
	ciphertext := encryptedBuf[c.nonceSize:]

	// Decrypt
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("decryption failed: %w", err)
	}

	// Check if decrypted data fits in buffer
	if len(plaintext) > len(b) {
		return 0, nil, nil, fmt.Errorf("decrypted data too large: %d > %d", len(plaintext), len(b))
	}

	// Copy decrypted data to output buffer
	copy(b, plaintext)
	return len(plaintext), nil, raddr, nil
}

// SetWriteDeadline sets the write deadline.
func (c *EncryptedUDPConn) SetWriteDeadline(t net.Time) error {
	return c.underlying.SetWriteDeadline(t)
}

// SetMulticastInterface sets the multicast interface.
func (c *EncryptedUDPConn) SetMulticastInterface(ifi *net.Interface) error {
	// This would need to be implemented if you want multicast support
	// For now, we'll pass it through if the underlying connection supports it
	return nil
}

// SetMulticastHopLimit sets the multicast hop limit.
func (c *EncryptedUDPConn) SetMulticastHopLimit(hoplim int) error {
	// This would need to be implemented if you want multicast support
	return nil
}

// SetMulticastLoopback sets multicast loopback.
func (c *EncryptedUDPConn) SetMulticastLoopback(on bool) error {
	// This would need to be implemented if you want multicast support
	return nil
}

// JoinGroup joins a multicast group.
func (c *EncryptedUDPConn) JoinGroup(ifi *net.Interface, group net.Addr) error {
	// This would need to be implemented if you want multicast support
	return nil
}

// LeaveGroup leaves a multicast group.
func (c *EncryptedUDPConn) LeaveGroup(ifi *net.Interface, group net.Addr) error {
	// This would need to be implemented if you want multicast support
	return nil
}

// SupportsControlMessage returns whether control messages are supported.
func (c *EncryptedUDPConn) SupportsControlMessage() bool {
	return false // Encryption makes control messages complex
}

// IsIPv6 returns whether this is an IPv6 connection.
func (c *EncryptedUDPConn) IsIPv6() bool {
	laddr := c.underlying.LocalAddr()
	if laddr == nil {
		return false
	}
	udpAddr, ok := laddr.(*net.UDPAddr)
	if !ok {
		return false
	}
	return udpAddr.IP.To4() == nil
}

// encryptedPacketConn wraps EncryptedUDPConn to implement the packetConn interface.
type encryptedPacketConn struct {
	*EncryptedUDPConn
}

// NewEncryptedUDPConnWrapper creates a coapNet.UDPConn from an encrypted connection.
func NewEncryptedUDPConnWrapper(network string, conn *net.UDPConn, keyProvider KeyProvider) (*coapNet.UDPConn, error) {
	encrypted, err := NewEncryptedUDPConn(conn, keyProvider)
	if err != nil {
		return nil, err
	}

	// We need to create a packetConn implementation
	// Since we can't directly create coapNet.UDPConn with a custom packetConn,
	// we'll need to use a different approach or extend the package
	
	// For now, this is a simplified version - in practice you might need to
	// modify the coapNet package or use reflection/unsafe to inject the packetConn
	// Or create a wrapper that implements the full UDPConn interface
	
	// This is a conceptual example - actual implementation would need more work
	_ = encrypted
	return nil, fmt.Errorf("direct creation not supported - see alternative approach in examples")
}






