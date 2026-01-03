package protocol

import (
	"net"
)

type PubAck struct {
	PubAckPacket
}

func (pa *PubAck) Encode() (net.Buffers, error) {
	return nil, nil
}
