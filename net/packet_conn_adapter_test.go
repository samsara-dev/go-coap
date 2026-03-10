package net

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// customAddr implements net.Addr with a non-UDPAddr type.
// Custom transports may return such types, causing readWithCfg to fail
// with "invalid srcAddr type" before the adapter fix.
type customAddr struct {
	network string
	addr    string
}

func (c *customAddr) Network() string { return c.network }
func (c *customAddr) String() string  { return c.addr }

// customAddrPacketConn wraps a net.PacketConn and returns customAddr instead of *net.UDPAddr.
type customAddrPacketConn struct {
	net.PacketConn
}

func (c *customAddrPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(b)
	if err != nil {
		return 0, nil, err
	}
	// Return custom address type instead of *net.UDPAddr.
	custom := &customAddr{network: addr.Network(), addr: addr.String()}
	return n, custom, nil
}

func TestCustomTransportPacketConnAdapter_ReadFrom_ConvertsCustomAddrToUDPAddr(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer conn.Close()

	wrapped := &customAddrPacketConn{PacketConn: conn}
	udpConn := NewUDPConnFromPacketConn(wrapped, conn, "udp4")
	defer udpConn.Close()

	// Send a packet from another conn so we have something to read.
	client, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err)
	defer client.Close()
	_, err = client.Write([]byte("hello"))
	require.NoError(t, err)

	// ReadWithOptions uses readWithCfg which type-asserts to *net.UDPAddr.
	// Before the fix, this would fail with "invalid srcAddr type *net.customAddr".
	var remoteAddr *net.UDPAddr
	buf := make([]byte, 1024)
	n, err := udpConn.ReadWithOptions(buf, WithGetRemoteAddr(&remoteAddr))
	require.NoError(t, err)
	assert.Greater(t, n, 0)
	assert.NotNil(t, remoteAddr, "remote addr should be populated after successful read")
	assert.Equal(t, client.LocalAddr().String(), remoteAddr.String())
}

// unresolvableAddr implements net.Addr with a network that cannot be resolved to UDP.
type unresolvableAddr struct {
	network string
	addr    string
}

func (u *unresolvableAddr) Network() string { return u.network }
func (u *unresolvableAddr) String() string  { return u.addr }

// unresolvablePacketConn returns an address type that cannot be converted to *net.UDPAddr.
type unresolvablePacketConn struct {
	net.PacketConn
}

func (u *unresolvablePacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, _, err := u.PacketConn.ReadFrom(b)
	if err != nil {
		return 0, nil, err
	}
	return n, &unresolvableAddr{network: "custom", addr: "invalid"}, nil
}

func TestCustomTransportPacketConnAdapter_ReadFrom_ReturnsClearErrorForUnresolvableAddr(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer conn.Close()

	wrapped := &unresolvablePacketConn{PacketConn: conn}
	udpConn := NewUDPConnFromPacketConn(wrapped, conn, "udp4")
	defer udpConn.Close()

	client, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err)
	defer client.Close()
	_, err = client.Write([]byte("hello"))
	require.NoError(t, err)

	buf := make([]byte, 1024)
	_, err = udpConn.ReadWithOptions(buf, WithGetRemoteAddr(new(*net.UDPAddr)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "addr type")
	assert.Contains(t, err.Error(), "need *net.UDPAddr")
	assert.Contains(t, err.Error(), "unresolvableAddr")
}

// nilAddrPacketConn returns (n, nil, nil) to simulate a custom PacketConn that
// returns nil addr with nil error, which would cause addrToUDPAddr to panic.
type nilAddrPacketConn struct {
	net.PacketConn
}

func (n *nilAddrPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	got, _, err := n.PacketConn.ReadFrom(b)
	if err != nil {
		return 0, nil, err
	}
	return got, nil, nil
}

func TestCustomTransportPacketConnAdapter_ReadFrom_NilAddrReturnsError(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer conn.Close()

	wrapped := &nilAddrPacketConn{PacketConn: conn}
	udpConn := NewUDPConnFromPacketConn(wrapped, conn, "udp4")
	defer udpConn.Close()

	client, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err)
	defer client.Close()
	_, err = client.Write([]byte("x"))
	require.NoError(t, err)

	buf := make([]byte, 1024)
	_, err = udpConn.ReadWithOptions(buf, WithGetRemoteAddr(new(*net.UDPAddr)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil address")
}

func TestCustomTransportPacketConnAdapter_WriteMulticast_ReturnsError(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer conn.Close()

	wrapped := &customAddrPacketConn{PacketConn: conn}
	udpConn := NewUDPConnFromPacketConn(wrapped, conn, "udp4")
	defer udpConn.Close()

	// WriteMulticast on custom packet conn must return error; otherwise writeToAddr would
	// use newPacketConnWithAddr(c.connection) and send data through raw unprotected UDP.
	raddr := &net.UDPAddr{IP: net.IPv4(224, 0, 1, 187), Port: 5683}
	err = udpConn.WriteMulticast(context.Background(), raddr, []byte("test"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WriteMulticast not supported on custom packet conn")
}
