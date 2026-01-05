package protocol

import (
	"bytes"
	"net"
)

type UnSubscribe struct {
	PacketID              uint16
	TopicFilters          []string
	UnSubscribeProperties *UnSubscribeProperties
}

// Note: `UserProperty` could be a direct property in `UnSubscribe`, since
// `UnSubscribeProperties` has only one property. However, I prefer code consistency
// and easier extensibility if the spec changes.
type UnSubscribeProperties struct {
	UserProperty []UserProperty
}

func (s *UnSubscribe) Encode() (net.Buffers, error) {
	var buf bytes.Buffer

	buf.WriteByte(TypeUnsubscribe<<4 | 0x02)

	vHeader, err := s.encodeVariableHeader()
	if err != nil {
		return nil, err
	}

	payload, err := s.encodePayload()
	if err != nil {
		return nil, err
	}

	remainingLength := len(vHeader) + len(payload)
	encodeVariableByteInteger(&buf, remainingLength)

	return net.Buffers{buf.Bytes(), vHeader, payload}, nil
}

func (s *UnSubscribe) encodeVariableHeader() ([]byte, error) {
	var buf bytes.Buffer

	if err := encodeUint16(&buf, s.PacketID); err != nil {
		return nil, err
	}

	if s.UnSubscribeProperties == nil {
		buf.WriteByte(0x00)
	} else {
		propBytes, err := s.UnSubscribeProperties.Encode()
		if err != nil {
			return nil, err
		}
		buf.Write(propBytes)
	}

	return buf.Bytes(), nil
}

func (s *UnSubscribe) encodePayload() ([]byte, error) {
	var buf bytes.Buffer

	for _, topic := range s.TopicFilters {
		if err := encodeString(&buf, topic); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (sp *UnSubscribeProperties) Encode() ([]byte, error) {
	var propsBuf bytes.Buffer

	for _, userProp := range sp.UserProperty {
		propsBuf.WriteByte(PropertyUserProperty)
		err := encodeString(&propsBuf, userProp.Key)
		if err != nil {
			return nil, err
		}
		err = encodeString(&propsBuf, userProp.Value)
		if err != nil {
			return nil, err
		}
	}

	var finalBuf bytes.Buffer
	encodeVariableByteInteger(&finalBuf, propsBuf.Len())
	finalBuf.Write(propsBuf.Bytes())

	return finalBuf.Bytes(), nil
}
