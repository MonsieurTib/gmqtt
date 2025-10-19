package protocol

import "bytes"

type PingReq struct{}

func (p *PingReq) Encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(TypePingReq << 4)
	encodeVariableByteInteger(&buf, 0)

	return buf.Bytes(), nil
}
