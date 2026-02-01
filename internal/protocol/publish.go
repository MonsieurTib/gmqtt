package protocol

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

const (
	FlagRetain = 0x01
	FlagDup    = 0x08
)

const (
	PubAckSuccess                     = 0x00
	PubAckNoMatchingSubscriber        = 0x10
	PubAckUnspecifiedError            = 0x80
	PubAckImplementationSpecificError = 0x83
	PubAckNotAuthorized               = 0x87
	PubAckTopicNameInvalid            = 0x90
	PubAckPacketIdentifierInUse       = 0x91
	PubAckPacketIdentifierNotFound    = 0x92
	PubAckQuotaExceed                 = 0x97
	PubAckPayloadFormatInvalid        = 0x99
)

type PublishOptions struct {
	Qos               byte
	Retain            bool
	Topic             string
	PublishProperties *PublishProperties
	Payload           []byte
}
type Publish struct {
	packetID          *uint16
	qos               byte
	retain            bool
	duplicate         bool
	topic             string
	publishProperties *PublishProperties
	payload           []byte
	acknowledgements  map[string]string
}

type PublishProperties struct {
	PayloadFormatIndicator *bool
	MessageExpiryInterval  *uint32
	TopicAlias             *uint16
	ResponseTopic          string
	CorrelationData        []byte
	UserProperty           []UserProperty
	SubscriptionIdentifier *uint32
	ContentType            string
}

func NewPublish(opt PublishOptions) (*Publish, error) {
	publish := &Publish{
		qos:               opt.Qos,
		retain:            opt.Retain,
		topic:             opt.Topic,
		publishProperties: opt.PublishProperties,
		payload:           opt.Payload,
	}
	if strings.ContainsAny(opt.Topic, "+#") {
		return nil, fmt.Errorf("invalid topic name: wildcards not allowed in PUBLISH")
	}

	return publish, nil
}

func (p *Publish) GetPacketID() uint16 {
	return *p.packetID
}

func (p *Publish) SetPacketID(packetID uint16) {
	p.packetID = &packetID
}

func (p *Publish) Topic() string {
	return p.topic
}

func (p *Publish) MarkAsDuplicated() {
	p.duplicate = true
}

func (p *Publish) Properties() *PublishProperties {
	return p.publishProperties
}

func (p *Publish) Encode() (net.Buffers, error) {
	var buf bytes.Buffer
	var header byte
	if p.retain {
		header |= FlagRetain
	}
	header |= p.qos << 1
	if p.duplicate {
		header |= FlagDup
	}
	header |= TypePublish << 4
	buf.WriteByte(header)

	vHeader, err := p.encodeVariableHeader()
	if err != nil {
		return nil, err
	}

	remainingLength := len(vHeader) + len(p.payload)

	encodeVariableByteInteger(&buf, remainingLength)
	return net.Buffers{buf.Bytes(), vHeader, p.payload}, nil
}

func (p *Publish) encodeVariableHeader() ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeString(&buf, p.topic); err != nil {
		return nil, err
	}

	if p.packetID != nil {
		if err := encodeUint16(&buf, *p.packetID); err != nil {
			return nil, err
		}
	}

	if p.publishProperties == nil {
		buf.WriteByte(0x00)
	} else {
		propBytes, err := p.publishProperties.Encode()
		if err != nil {
			return nil, err
		}
		buf.Write(propBytes)
	}

	return buf.Bytes(), nil
}

func (p *Publish) encodePayload() ([]byte, error) {
	return p.payload, nil
}

func (pp *PublishProperties) Encode() ([]byte, error) {
	var propsBuf bytes.Buffer

	if pp.PayloadFormatIndicator != nil {
		propsBuf.WriteByte(PropertyPayloadFormatIndicator)
		if *pp.PayloadFormatIndicator {
			propsBuf.WriteByte(0x01)
		} else {
			propsBuf.WriteByte(0x00)
		}
	}

	if pp.MessageExpiryInterval != nil {
		propsBuf.WriteByte(PropertyMessageExpiryInterval)
		err := encodeUint32(&propsBuf, *pp.MessageExpiryInterval)
		if err != nil {
			return nil, err
		}
	}

	if pp.TopicAlias != nil {
		propsBuf.WriteByte(PropertyTopicAlias)
		err := encodeUint16(&propsBuf, *pp.TopicAlias)
		if err != nil {
			return nil, err
		}
	}

	if pp.ResponseTopic != "" {
		propsBuf.WriteByte(PropertyResponseTopic)
		err := encodeString(&propsBuf, pp.ResponseTopic)
		if err != nil {
			return nil, err
		}
	}

	if len(pp.CorrelationData) > 0 {
		propsBuf.WriteByte(PropertyCorrelationData)
		err := encodeBinary(&propsBuf, pp.CorrelationData)
		if err != nil {
			return nil, err
		}
	}

	for _, userProp := range pp.UserProperty {
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

	if pp.SubscriptionIdentifier != nil {
		propsBuf.WriteByte(PropertySubscriptionIdentifier)
		encodeVariableByteInteger(&propsBuf, int(*pp.SubscriptionIdentifier))
	}

	if pp.ContentType != "" {
		propsBuf.WriteByte(PropertyContentType)
		err := encodeString(&propsBuf, pp.ContentType)
		if err != nil {
			return nil, err
		}
	}

	var finalBuf bytes.Buffer
	encodeVariableByteInteger(&finalBuf, propsBuf.Len())
	finalBuf.Write(propsBuf.Bytes())

	return finalBuf.Bytes(), nil
}
