package protocol

import (
	"bytes"
	"net"
)

type PubRel struct {
	PubAckPacket
}

func (ap *PubRel) Encode() (net.Buffers, error) {
	var buf bytes.Buffer

	buf.WriteByte(TypePubRel<<4 | 1<<1)

	vHeader, err := ap.encodeVariableHeader()
	if err != nil {
		return nil, err
	}

	encodeVariableByteInteger(&buf, len(vHeader))

	return net.Buffers{buf.Bytes(), vHeader}, nil
}

func NewPubRel(packetID uint16, reasonCode byte, properties *PubAckProperties) (*PubRel, error) {
	pubRel := &PubRel{
		PubAckPacket: PubAckPacket{
			PacketID:   packetID,
			ReasonCode: reasonCode,
			Properties: properties,
		},
	}

	return pubRel, nil
}

func (ap *PubRel) encodeVariableHeader() ([]byte, error) {
	var buf bytes.Buffer

	if err := encodeUint16(&buf, ap.PacketID); err != nil {
		return nil, err
	}

	buf.WriteByte(ap.ReasonCode)

	if ap.Properties == nil {
		buf.WriteByte(0x00)
	} else {
		propBytes, err := ap.Properties.Encode()
		if err != nil {
			return nil, err
		}
		buf.Write(propBytes)
	}

	return buf.Bytes(), nil
}
