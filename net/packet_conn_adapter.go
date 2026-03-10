// Package net provides packet conn adapter for custom transports.
package net

import (
	"fmt"
	"net"
	"time"
)

// customTransportPacketConnAdapter adapts net.PacketConn to the internal packetConn interface.
// Used by NewUDPConnFromPacketConn for custom packet conns (e.g. encrypted or wrapped transports).
type customTransportPacketConnAdapter struct {
	pc     net.PacketConn
	isIPv6 bool
}

func newCustomTransportPacketConnAdapter(pc net.PacketConn, localAddr net.Addr) *customTransportPacketConnAdapter {
	isIPv6 := false
	if addr, ok := localAddr.(*net.UDPAddr); ok && addr.IP != nil {
		isIPv6 = IsIPv6(addr.IP)
	}
	return &customTransportPacketConnAdapter{
		pc:     pc,
		isIPv6: isIPv6,
	}
}

// addrToUDPAddr converts net.Addr to *net.UDPAddr. Returns the address unchanged if it is
// already *net.UDPAddr, or attempts to resolve when Network() is "udp"/"udp4"/"udp6".
// Custom PacketConns that return non-UDPAddr types cause readWithCfg to fail with
// "invalid srcAddr type"; this normalization allows transports that return resolvable
// addresses to work with go-coap.
func addrToUDPAddr(addr net.Addr) (*net.UDPAddr, error) {
	if addr == nil {
		return nil, fmt.Errorf("custom PacketConn returned nil address")
	}
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		return udpAddr, nil
	}
	netw := addr.Network()
	if netw == "udp" || netw == "udp4" || netw == "udp6" {
		resolved, err := net.ResolveUDPAddr(netw, addr.String())
		if err == nil {
			return resolved, nil
		}
	}
	return nil, fmt.Errorf("addr type %T; need *net.UDPAddr or resolvable udp", addr)
}

func (a *customTransportPacketConnAdapter) ReadFrom(b []byte) (int, *ControlMessage, net.Addr, error) {
	n, addr, err := a.pc.ReadFrom(b)
	if err != nil {
		return -1, nil, nil, err
	}
	// addr is per-packet (source of this read), not per-connection; cannot cache.
	udpAddr, err := addrToUDPAddr(addr)
	if err != nil {
		return -1, nil, nil, err
	}
	return n, nil, udpAddr, nil
}

func (a *customTransportPacketConnAdapter) WriteTo(b []byte, cm *ControlMessage, dst net.Addr) (int, error) {
	return a.pc.WriteTo(b, dst)
}

func (a *customTransportPacketConnAdapter) SetWriteDeadline(t time.Time) error {
	return a.pc.SetWriteDeadline(t)
}

func (a *customTransportPacketConnAdapter) SetMulticastInterface(ifi *net.Interface) error {
	return nil
}

func (a *customTransportPacketConnAdapter) SetMulticastHopLimit(hoplim int) error {
	return nil
}

func (a *customTransportPacketConnAdapter) SetMulticastLoopback(on bool) error {
	return nil
}

func (a *customTransportPacketConnAdapter) JoinGroup(ifi *net.Interface, group net.Addr) error {
	return nil
}

func (a *customTransportPacketConnAdapter) LeaveGroup(ifi *net.Interface, group net.Addr) error {
	return nil
}

func (a *customTransportPacketConnAdapter) SupportsControlMessage() bool {
	return false
}

func (a *customTransportPacketConnAdapter) IsIPv6() bool {
	return a.isIPv6
}

// Close closes the underlying net.PacketConn. Required for proper cleanup when
// using NewUDPConnFromPacketConn with custom transports that hold resources
// like goroutines, buffers, or encryption state.
func (a *customTransportPacketConnAdapter) Close() error {
	return a.pc.Close()
}
