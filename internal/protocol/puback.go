package protocol

import (
	"net"
)

type PubAck struct {
	AckPacket
}

func (pa *PubAck) Encode() (net.Buffers, error) {
	return nil, nil
}
