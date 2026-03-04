// Package net - connUDP_packetconn.go provides NewUDPConnFromPacketConn for
// creating a UDPConn that reads from a custom net.PacketConn (e.g. a decrypting
// wrapper) while using a separate underlying UDPConn for writes.
package net

import (
	"net"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// NewUDPConnFromPacketConn creates a UDPConn that reads from readConn and uses
// underlyingUDPConn for writes.
func NewUDPConnFromPacketConn(network string, readConn net.PacketConn, underlyingUDPConn *net.UDPConn, opts ...UDPOption) *UDPConn {
	cfg := DefaultUDPConnConfig
	for _, o := range opts {
		o.ApplyUDP(&cfg)
	}
	laddr := underlyingUDPConn.LocalAddr()
	addr, ok := laddr.(*net.UDPAddr)
	if !ok {
		panic("underlyingUDPConn must have *net.UDPAddr as LocalAddr")
	}
	var pc packetConn
	if IsIPv6(addr.IP) {
		pc = newPacketConnIPv6(ipv6.NewPacketConn(readConn))
	} else {
		pc = newPacketConnIPv4(ipv4.NewPacketConn(readConn))
	}
	return &UDPConn{
		network:    network,
		connection: underlyingUDPConn,
		packetConn: pc,
		errors:     cfg.Errors,
	}
}
