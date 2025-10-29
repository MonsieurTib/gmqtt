package protocol

import (
	"bytes"
	"net"
)

type PingReq struct{}

func (p *PingReq) Encode() (net.Buffers, error) {
	var buf bytes.Buffer
	buf.WriteByte(TypePingReq << 4)
	encodeVariableByteInteger(&buf, 0)

	return net.Buffers{buf.Bytes()}, nil
}
