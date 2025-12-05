package aesgcm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

// AESGCMConn wraps a UDP connection with AES-GCM encryption.
// It implements net.Conn to work seamlessly with the CoAP library.
type AESGCMConn struct {
	conn       *net.UDPConn
	raddr      *net.UDPAddr
	aead       cipher.AEAD
	nonceSize  int
	closed     bool
	closedLock sync.RWMutex
}

// NewAESGCMConn creates a new AES-GCM encrypted connection.
// key must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256 respectively.
func NewAESGCMConn(conn *net.UDPConn, raddr *net.UDPAddr, key []byte) (*AESGCMConn, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &AESGCMConn{
		conn:      conn,
		raddr:     raddr,
		aead:      aead,
		nonceSize: aead.NonceSize(),
	}, nil
}

// Read reads encrypted data from the connection and decrypts it.
func (c *AESGCMConn) Read(b []byte) (n int, err error) {
	c.closedLock.RLock()
	closed := c.closed
	c.closedLock.RUnlock()
	if closed {
		return 0, net.ErrClosed
	}

	// Read encrypted packet from UDP
	encrypted := make([]byte, 4096) // Adjust size as needed
	n, _, err = c.conn.ReadFromUDP(encrypted)
	if err != nil {
		return 0, err
	}
	encrypted = encrypted[:n]

	// Check minimum size (nonce + ciphertext + tag)
	if len(encrypted) < c.nonceSize {
		return 0, errors.New("encrypted data too short")
	}

	// Extract nonce and ciphertext
	nonce := encrypted[:c.nonceSize]
	ciphertext := encrypted[c.nonceSize:]

	// Decrypt
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return 0, fmt.Errorf("decryption failed: %w", err)
	}

	// Copy decrypted data to output buffer
	if len(plaintext) > len(b) {
		return 0, errors.New("decrypted data too large for buffer")
	}
	copy(b, plaintext)
	return len(plaintext), nil
}

// Write encrypts data and writes it to the connection.
func (c *AESGCMConn) Write(b []byte) (n int, err error) {
	c.closedLock.RLock()
	closed := c.closed
	c.closedLock.RUnlock()
	if closed {
		return 0, net.ErrClosed
	}

	// Generate random nonce
	nonce := make([]byte, c.nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return 0, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt plaintext
	ciphertext := c.aead.Seal(nonce, nonce, b, nil)

	// Write encrypted packet to UDP
	written, err := c.conn.WriteToUDP(ciphertext, c.raddr)
	if err != nil {
		return 0, err
	}

	// Return the number of plaintext bytes written (not encrypted bytes)
	return len(b), nil
}

// Close closes the underlying connection.
func (c *AESGCMConn) Close() error {
	c.closedLock.Lock()
	defer c.closedLock.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

// LocalAddr returns the local network address.
func (c *AESGCMConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *AESGCMConn) RemoteAddr() net.Addr {
	return c.raddr
}

// SetDeadline sets the read and write deadlines.
func (c *AESGCMConn) SetDeadline(t net.Time) error {
	return c.conn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline.
func (c *AESGCMConn) SetReadDeadline(t net.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline.
func (c *AESGCMConn) SetWriteDeadline(t net.Time) error {
	return c.conn.SetWriteDeadline(t)
}






