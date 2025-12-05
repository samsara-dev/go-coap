package aesgcm

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/plgd-dev/go-coap/v3/message/pool"
	coapNet "github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/udp/client"
	"github.com/plgd-dev/go-coap/v3/udp/coder"
)

// Session implements the udp/client.Session interface for AES-GCM encrypted connections.
// This is a custom session type - you don't need dtlsServer.NewSession!
type Session struct {
	ctx atomic.Pointer[context.Context]
	cancel context.CancelFunc
	
	connection *coapNet.Conn
	
	done chan struct{}
	mutex sync.Mutex
	onClose []func()
	
	maxMessageSize uint32
	mtu uint16
	closeSocket bool
}

// NewSession creates a new AES-GCM session.
func NewSession(
	ctx context.Context,
	connection *coapNet.Conn,
	maxMessageSize uint32,
	mtu uint16,
	closeSocket bool,
) *Session {
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{
		cancel:         cancel,
		connection:     connection,
		maxMessageSize: maxMessageSize,
		mtu:            mtu,
		closeSocket:    closeSocket,
		done:           make(chan struct{}),
	}
	s.ctx.Store(&ctx)
	return s
}

func (s *Session) Context() context.Context {
	return *s.ctx.Load()
}

func (s *Session) Close() error {
	s.cancel()
	if s.closeSocket {
		return s.connection.Close()
	}
	return nil
}

func (s *Session) MaxMessageSize() uint32 {
	return s.maxMessageSize
}

func (s *Session) RemoteAddr() net.Addr {
	return s.connection.RemoteAddr()
}

func (s *Session) LocalAddr() net.Addr {
	return s.connection.LocalAddr()
}

func (s *Session) NetConn() net.Conn {
	return s.connection.NetConn()
}

func (s *Session) WriteMessage(req *pool.Message) error {
	// Marshal CoAP message to plain bytes
	data, err := req.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		return fmt.Errorf("cannot marshal: %w", err)
	}
	// Write to connection (encryption happens in AESGCMConn.Write)
	err = s.connection.WriteWithContext(req.Context(), data)
	if err != nil {
		return fmt.Errorf("cannot write to connection: %w", err)
	}
	return nil
}

// WriteMulticastMessage is not supported for encrypted connections.
func (s *Session) WriteMulticastMessage(*pool.Message, *net.UDPAddr, ...coapNet.MulticastOption) error {
	return fmt.Errorf("multicast messages not supported for encrypted connections")
}

func (s *Session) Run(cc *client.Conn) (err error) {
	defer func() {
		err1 := s.Close()
		if err == nil {
			err = err1
		}
		s.shutdown()
	}()
	m := make([]byte, s.mtu)
	for {
		readBuf := m
		readLen, err := s.connection.ReadWithContext(s.Context(), readBuf)
		if err != nil {
			return fmt.Errorf("cannot read from connection: %w", err)
		}
		readBuf = readBuf[:readLen]
		// Decryption happens in AESGCMConn.Read, so this is plain CoAP bytes
		err = cc.Process(nil, readBuf)
		if err != nil {
			return err
		}
	}
}

func (s *Session) AddOnClose(f func()) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.onClose = append(s.onClose, f)
}

func (s *Session) SetContextValue(key interface{}, val interface{}) {
	ctx := context.WithValue(s.Context(), key, val)
	s.ctx.Store(&ctx)
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) shutdown() {
	defer close(s.done)
	s.mutex.Lock()
	onClose := s.onClose
	s.onClose = nil
	s.mutex.Unlock()
	for _, f := range onClose {
		f()
	}
}






