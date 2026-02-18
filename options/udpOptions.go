package options

import (
	"time"

	dtlsServer "github.com/plgd-dev/go-coap/v3/dtls/server"
	udpClient "github.com/plgd-dev/go-coap/v3/udp/client"
	udpServer "github.com/plgd-dev/go-coap/v3/udp/server"
)

// TransmissionOpt transmission options.
type TransmissionOpt struct {
	transmissionNStart             uint32
	transmissionAcknowledgeTimeout time.Duration
	transmissionMaxRetransmit      uint32
}

func (o TransmissionOpt) UDPServerApply(cfg *udpServer.Config) {
	cfg.TransmissionNStart = o.transmissionNStart
	cfg.TransmissionAcknowledgeTimeout = o.transmissionAcknowledgeTimeout
	cfg.TransmissionMaxRetransmit = o.transmissionMaxRetransmit
}

func (o TransmissionOpt) DTLSServerApply(cfg *dtlsServer.Config) {
	cfg.TransmissionNStart = o.transmissionNStart
	cfg.TransmissionAcknowledgeTimeout = o.transmissionAcknowledgeTimeout
	cfg.TransmissionMaxRetransmit = o.transmissionMaxRetransmit
}

func (o TransmissionOpt) UDPClientApply(cfg *udpClient.Config) {
	cfg.TransmissionNStart = o.transmissionNStart
	cfg.TransmissionAcknowledgeTimeout = o.transmissionAcknowledgeTimeout
	cfg.TransmissionMaxRetransmit = o.transmissionMaxRetransmit
}

// WithTransmission set options for (re)transmission for Confirmable message-s.
func WithTransmission(transmissionNStart uint32,
	transmissionAcknowledgeTimeout time.Duration,
	transmissionMaxRetransmit uint32,
) TransmissionOpt {
	return TransmissionOpt{
		transmissionNStart:                   transmissionNStart,
		transmissionAcknowledgeTimeout:       transmissionAcknowledgeTimeout,
		transmissionMaxRetransmit:            transmissionMaxRetransmit,
	}
}

// TransmissionBackoffOpt configures retransmission backoff behavior.
type TransmissionBackoffOpt struct {
	acknowledgeRandomFactor  float64
	exponentialBackoffEnable bool
}

func (o TransmissionBackoffOpt) UDPServerApply(cfg *udpServer.Config) {
	cfg.TransmissionAcknowledgeRandomFactor = o.acknowledgeRandomFactor
	cfg.TransmissionExponentialBackoffEnable = o.exponentialBackoffEnable
}

func (o TransmissionBackoffOpt) DTLSServerApply(cfg *dtlsServer.Config) {
	cfg.TransmissionAcknowledgeRandomFactor = o.acknowledgeRandomFactor
	cfg.TransmissionExponentialBackoffEnable = o.exponentialBackoffEnable
}

func (o TransmissionBackoffOpt) UDPClientApply(cfg *udpClient.Config) {
	cfg.TransmissionAcknowledgeRandomFactor = o.acknowledgeRandomFactor
	cfg.TransmissionExponentialBackoffEnable = o.exponentialBackoffEnable
}

// WithTransmissionBackoff configures the retransmission backoff strategy.
// When exponentialBackoffEnable is true, retransmission timeouts use exponential
// backoff (doubling on each retransmit) as specified in RFC 7252 Section 4.2,
// rather than linear multiples of acknowledgeTimeout.
// The acknowledgeRandomFactor introduces jitter into the initial timeout. Per RFC 7252,
// the initial timeout is chosen randomly between acknowledgeTimeout and
// acknowledgeTimeout * acknowledgeRandomFactor. The default value per the RFC is 1.5.
// A value of 1.0 disables randomization.
func WithTransmissionBackoff(
	acknowledgeRandomFactor float64,
	exponentialBackoffEnable bool,
) TransmissionBackoffOpt {
	return TransmissionBackoffOpt{
		acknowledgeRandomFactor:  acknowledgeRandomFactor,
		exponentialBackoffEnable: exponentialBackoffEnable,
	}
}

// MTUOpt transmission options.
type MTUOpt struct {
	mtu uint16
}

func (o MTUOpt) UDPServerApply(cfg *udpServer.Config) {
	cfg.MTU = o.mtu
}

func (o MTUOpt) DTLSServerApply(cfg *dtlsServer.Config) {
	cfg.MTU = o.mtu
}

func (o MTUOpt) UDPClientApply(cfg *udpClient.Config) {
	cfg.MTU = o.mtu
}

// Setup MTU unit
func WithMTU(mtu uint16) MTUOpt {
	return MTUOpt{
		mtu: mtu,
	}
}
