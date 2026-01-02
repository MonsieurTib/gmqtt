package protocol

import (
	"bytes"
	"net"
)

type Subscribe struct {
	PacketID            uint16
	Subscriptions       []Subscription
	SubscribeProperties *SubscribeProperties
}

type Subscription struct {
	TopicFilter       string
	QoS               byte
	NoLocal           bool
	RetainAsPublished bool
	RetainHandling    byte
}

type SubscribeProperties struct {
	SubscriptionIdentifier *uint32
	UserProperty           []UserProperty
}

func (s *Subscribe) Encode() (net.Buffers, error) {
	var buf bytes.Buffer

	buf.WriteByte(TypeSubscribe<<4 | 0x02)

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

func (s *Subscribe) encodeVariableHeader() ([]byte, error) {
	var buf bytes.Buffer

	if err := encodeUint16(&buf, s.PacketID); err != nil {
		return nil, err
	}

	if s.SubscribeProperties == nil {
		buf.WriteByte(0x00)
	} else {
		propBytes, err := s.SubscribeProperties.Encode()
		if err != nil {
			return nil, err
		}
		buf.Write(propBytes)
	}

	return buf.Bytes(), nil
}

func (s *Subscribe) encodePayload() ([]byte, error) {
	var buf bytes.Buffer

	for _, sub := range s.Subscriptions {
		if err := encodeString(&buf, sub.TopicFilter); err != nil {
			return nil, err
		}

		var options byte
		options |= sub.QoS & 0x03
		if sub.NoLocal {
			options |= 0x04
		}
		if sub.RetainAsPublished {
			options |= 0x08
		}
		options |= (sub.RetainHandling & 0x03) << 4

		buf.WriteByte(options)
	}

	return buf.Bytes(), nil
}

func (sp *SubscribeProperties) Encode() ([]byte, error) {
	var propsBuf bytes.Buffer

	if sp.SubscriptionIdentifier != nil {
		propsBuf.WriteByte(PropertySubscriptionIdentifier)
		encodeVariableByteInteger(&propsBuf, int(*sp.SubscriptionIdentifier))
	}

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
