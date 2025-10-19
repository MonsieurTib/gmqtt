package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type Disconnect struct {
	ReasonCode byte
	Properties *DisconnectProperties
}

type DisconnectProperties struct {
	SessionExpiryInterval *uint32
	ReasonString          string
	UserProperty          []UserProperty
	ServerReference       string
}

const (
	DisconnectNormalDisconnection                 = 0x00
	DisconnectDisconnectWithWillMessage           = 0x04
	DisconnectUnspecifiedError                    = 0x80
	DisconnectMalformedPacket                     = 0x81
	DisconnectProtocolError                       = 0x82
	DisconnectImplementationSpecificError         = 0x83
	DisconnectNotAuthorized                       = 0x87
	DisconnectServerBusy                          = 0x89
	DisconnectServerShuttingDown                  = 0x8B
	DisconnectKeepAliveTimeout                    = 0x8D
	DisconnectSessionTakenOver                    = 0x8E
	DisconnectTopicFilterInvalid                  = 0x8F
	DisconnectTopicNameInvalid                    = 0x90
	DisconnectReceiveMaximumExceeded              = 0x93
	DisconnectTopicAliasInvalid                   = 0x94
	DisconnectPacketTooLarge                      = 0x95
	DisconnectMessageRateTooHigh                  = 0x96
	DisconnectQuotaExceeded                       = 0x97
	DisconnectAdministrativeAction                = 0x98
	DisconnectPayloadFormatInvalid                = 0x99
	DisconnectRetainNotSupported                  = 0x9A
	DisconnectQoSNotSupported                     = 0x9B
	DisconnectUseAnotherServer                    = 0x9C
	DisconnectServerMoved                         = 0x9D
	DisconnectSharedSubscriptionsNotSupported     = 0x9E
	DisconnectConnectionRateExceeded              = 0x9F
	DisconnectMaximumConnectTime                  = 0xA0
	DisconnectSubscriptionIdentifiersNotSupported = 0xA1
	DisconnectWildcardSubscriptionsNotSupported   = 0xA2
)

func NewDisconnect(reasonCode byte, properties *DisconnectProperties) *Disconnect {
	return &Disconnect{
		ReasonCode: reasonCode,
		Properties: properties,
	}
}

func (c *Disconnect) Encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(TypeDisconnect << 4)

	header, err := c.encodeVariableHeader()
	if err != nil {
		return nil, err
	}

	encodeVariableByteInteger(&buf, len(header))
	buf.Write(header)

	return buf.Bytes(), nil
}

func (c *Disconnect) encodeVariableHeader() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(c.ReasonCode)

	if c.Properties == nil {
		buf.WriteByte(0x00)
	} else {
		propBytes, err := c.Properties.Encode()
		if err != nil {
			return nil, err
		}
		buf.Write(propBytes)
	}

	return buf.Bytes(), nil
}

func (dp *DisconnectProperties) Encode() ([]byte, error) {
	var propsBuf bytes.Buffer

	if dp.SessionExpiryInterval != nil {
		propsBuf.WriteByte(PropertySessionExpiryInterval)
		err := encodeUint32(&propsBuf, *dp.SessionExpiryInterval)
		if err != nil {
			return nil, err
		}
	}

	if dp.ReasonString != "" {
		propsBuf.WriteByte(PropertyReasonString)
		err := encodeString(&propsBuf, dp.ReasonString)
		if err != nil {
			return nil, err
		}
	}

	for _, userProp := range dp.UserProperty {
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

	if dp.ServerReference != "" {
		propsBuf.WriteByte(PropertyServerReference)
		err := encodeString(&propsBuf, dp.ServerReference)
		if err != nil {
			return nil, err
		}
	}

	var finalBuf bytes.Buffer
	encodeVariableByteInteger(&finalBuf, propsBuf.Len())
	finalBuf.Write(propsBuf.Bytes())

	return finalBuf.Bytes(), nil
}

func (c *Disconnect) Decode(reader io.Reader) error {
	remainingLength, err := decodeVariableByteInteger(reader)
	if err != nil {
		return fmt.Errorf("error decoding remaining length on disconnect: %w", err)
	}

	buf := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return fmt.Errorf("error reading disconnect packet: %w", err)
	}

	if len(buf) == 0 {
		c.ReasonCode = 0
		return nil
	}

	c.ReasonCode = buf[0]

	if remainingLength > 1 {
		c.Properties = &DisconnectProperties{}
		propsBuf := bytes.NewBuffer(buf[1:])
		err := c.Properties.decode(propsBuf)
		if err != nil {
			return fmt.Errorf("error decoding disconnect properties: %w", err)
		}
	}

	return nil
}

func (dp *DisconnectProperties) decode(buf *bytes.Buffer) error {
	propLen, err := decodeVariableByteIntegerFromBuffer(buf)
	if err != nil {
		return fmt.Errorf("error decoding properties length: %w", err)
	}

	propData := make([]byte, propLen)
	if _, err := io.ReadFull(buf, propData); err != nil {
		return fmt.Errorf("error reading properties data: %w", err)
	}

	propBuf := bytes.NewBuffer(propData)
	for propBuf.Len() > 0 {
		propID, err := propBuf.ReadByte()
		if err != nil {
			return fmt.Errorf("error reading property identifier: %w", err)
		}

		switch propID {
		case PropertySessionExpiryInterval:
			sessionExpiry, err := decodeUint32(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding session expiry interval: %w", err)
			}
			dp.SessionExpiryInterval = &sessionExpiry
		case PropertyReasonString:
			reasonString, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding reason string: %w", err)
			}
			dp.ReasonString = reasonString
		case PropertyUserProperty:
			key, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property key: %w", err)
			}
			value, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property value: %w", err)
			}
			if dp.UserProperty == nil {
				dp.UserProperty = make([]UserProperty, 0)
			}
			dp.UserProperty = append(dp.UserProperty, UserProperty{Key: key, Value: value})
		case PropertyServerReference:
			serverRef, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding server reference: %w", err)
			}
			dp.ServerReference = serverRef
		default:
			return fmt.Errorf("unsupported property identifier: 0x%02X", propID)
		}
	}

	return nil
}
