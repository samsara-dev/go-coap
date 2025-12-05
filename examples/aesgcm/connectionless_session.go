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
	"sync/atomic"

	"github.com/plgd-dev/go-coap/v3/message/pool"
	coapNet "github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/udp/client"
	"github.com/plgd-dev/go-coap/v3/udp/coder"
	udpServer "github.com/plgd-dev/go-coap/v3/udp/server"
)

// ConnectionlessEncryptedSession implements udp/client.Session with encryption.
// It's similar to udp/server.Session but adds encryption/decryption at the packet level.
type ConnectionlessEncryptedSession struct {
	onClose []func()

	ctx atomic.Pointer[context.Context]

	doneCtx    context.Context
	connection *coapNet.UDPConn
	doneCancel context.CancelFunc

	cancel context.CancelFunc
	raddr  *net.UDPAddr

	mutex          sync.Mutex
	maxMessageSize uint32
	mtu            uint16

	closeSocket bool
	keyProvider  KeyProvider
}

// NewConnectionlessEncryptedSession creates a new encrypted session.
func NewConnectionlessEncryptedSession(
	ctx context.Context,
	doneCtx context.Context,
	connection *coapNet.UDPConn,
	raddr *net.UDPAddr,
	maxMessageSize uint32,
	mtu uint16,
	closeSocket bool,
	keyProvider KeyProvider,
) *ConnectionlessEncryptedSession {
	ctx, cancel := context.WithCancel(ctx)
	doneCtx, doneCancel := context.WithCancel(doneCtx)

	s := &ConnectionlessEncryptedSession{
		cancel:         cancel,
		connection:     connection,
		raddr:          raddr,
		maxMessageSize: maxMessageSize,
		mtu:            mtu,
		closeSocket:    closeSocket,
		doneCtx:        doneCtx,
		doneCancel:     doneCancel,
		keyProvider:    keyProvider,
	}
	s.ctx.Store(&ctx)
	return s
}

func (s *ConnectionlessEncryptedSession) Context() context.Context {
	return *s.ctx.Load()
}

func (s *ConnectionlessEncryptedSession) SetContextValue(key interface{}, val interface{}) {
	ctx := context.WithValue(s.Context(), key, val)
	s.ctx.Store(&ctx)
}

func (s *ConnectionlessEncryptedSession) Done() <-chan struct{} {
	return s.doneCtx.Done()
}

func (s *ConnectionlessEncryptedSession) AddOnClose(f func()) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.onClose = append(s.onClose, f)
}

func (s *ConnectionlessEncryptedSession) popOnClose() []func() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	tmp := s.onClose
	s.onClose = nil
	return tmp
}

func (s *ConnectionlessEncryptedSession) shutdown() {
	defer s.doneCancel()
	for _, f := range s.popOnClose() {
		f()
	}
}

func (s *ConnectionlessEncryptedSession) Close() error {
	s.cancel()
	if s.closeSocket {
		return s.connection.Close()
	}
	return nil
}

func (s *ConnectionlessEncryptedSession) MaxMessageSize() uint32 {
	return s.maxMessageSize
}

func (s *ConnectionlessEncryptedSession) RemoteAddr() net.Addr {
	return s.raddr
}

func (s *ConnectionlessEncryptedSession) LocalAddr() net.Addr {
	return s.connection.LocalAddr()
}

func (s *ConnectionlessEncryptedSession) NetConn() net.Conn {
	return s.connection.NetConn()
}

// WriteMessage encrypts the message before writing.
func (s *ConnectionlessEncryptedSession) WriteMessage(req *pool.Message) error {
	// Marshal CoAP message to plain bytes
	data, err := req.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		return fmt.Errorf("cannot marshal: %w", err)
	}

	// Get encryption key for remote address
	raddr := s.RemoteAddr().(*net.UDPAddr)
	key, err := s.keyProvider.GetKey(raddr)
	if err != nil {
		return fmt.Errorf("cannot get encryption key: %w", err)
	}

	// Encrypt the data
	encryptedData, err := s.encrypt(data, key)
	if err != nil {
		return fmt.Errorf("cannot encrypt: %w", err)
	}

	// Write encrypted data to connection
	return s.connection.WriteWithOptions(
		encryptedData,
		coapNet.WithContext(req.Context()),
		coapNet.WithRemoteAddr(raddr),
		coapNet.WithControlMessage(req.ControlMessage()),
	)
}

// Run reads encrypted packets, decrypts them, and processes them.
func (s *ConnectionlessEncryptedSession) Run(cc *client.Conn) (err error) {
	defer func() {
		err1 := s.Close()
		if err == nil {
			err = err1
		}
		s.shutdown()
	}()

	m := make([]byte, s.mtu+256) // Extra space for encryption overhead (nonce + tag)
	for {
		buf := m
		var cm *coapNet.ControlMessage

		// Read encrypted packet
		// Note: ReadWithOptions doesn't return remote addr directly for connection-oriented UDP
		// For connectionless, we use the session's raddr (which is set per connection)
		n, err := s.connection.ReadWithOptions(
			buf,
			coapNet.WithContext(s.Context()),
			coapNet.WithGetControlMessage(&cm),
		)
		if err != nil {
			return err
		}
		buf = buf[:n]

		// Get encryption key for remote address
		// For connectionless UDP, each session is per client address
		// So we use the session's raddr
		key, err := s.keyProvider.GetKey(s.raddr)
		if err != nil {
			return fmt.Errorf("cannot get decryption key for %v: %w", keyAddr, err)
		}

		// Decrypt the data
		plaintext, err := s.decrypt(buf, key)
		if err != nil {
			return fmt.Errorf("cannot decrypt: %w", err)
		}

		// Process decrypted CoAP message
		err = cc.Process(cm, plaintext)
		if err != nil {
			return err
		}
	}
}

// WriteMulticastMessage sends encrypted multicast message.
func (s *ConnectionlessEncryptedSession) WriteMulticastMessage(req *pool.Message, address *net.UDPAddr, opts ...coapNet.MulticastOption) error {
	// Marshal CoAP message
	data, err := req.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		return fmt.Errorf("cannot marshal: %w", err)
	}

	// Get encryption key for multicast address
	key, err := s.keyProvider.GetKey(address)
	if err != nil {
		return fmt.Errorf("cannot get encryption key: %w", err)
	}

	// Encrypt
	encryptedData, err := s.encrypt(data, key)
	if err != nil {
		return fmt.Errorf("cannot encrypt: %w", err)
	}

	return s.connection.WriteMulticast(req.Context(), address, encryptedData, opts...)
}

// encrypt encrypts data using AES-GCM.
func (s *ConnectionlessEncryptedSession) encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt decrypts data using AES-GCM.
func (s *ConnectionlessEncryptedSession) decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

