package client

import "errors"

var (
	ErrRetransmitLimitReached = errors.New("retransmit limit reached")
)
