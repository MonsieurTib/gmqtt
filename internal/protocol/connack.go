package protocol

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
)

const (
	ConnackSuccess                    = 0x00
	ConnackUnspecifiedError           = 0x80
	ConnackMalformedPacket            = 0x81
	ConnackProtocolError              = 0x82
	ConnackImplementationError        = 0x83
	ConnackUnsupportedProtocolVersion = 0x84
	ConnackClientIdentifierNotValid   = 0x85
	ConnackBadUsernameOrPassword      = 0x86
	ConnackNotAuthorized              = 0x87
	ConnackServerUnavailable          = 0x88
	ConnackServerBusy                 = 0x89
	ConnackBanned                     = 0x8A
	ConnackBadAuthenticationMethod    = 0x8C
	ConnackTopicNameInvalid           = 0x90
	ConnackPacketToLarge              = 0x95
	ConnackQuotaExceeded              = 0x97
	ConnackPayloadFormatInvalid       = 0x99
	ConnackRetainNotSupported         = 0x9A
	ConnackQosNotSupported            = 0x9B
	ConnackUseAnotherServer           = 0x9C
	ConnackServerMoved                = 0x9D
	ConnackConnectionRateExceeded     = 0x9F
)

type ConnAckProperties struct {
	SessionExpiryInterval           *uint32
	ReceiveMaximum                  *uint16
	MaximumQoS                      *byte
	RetainAvailable                 *byte
	MaximumPacketSize               *uint32
	AssignedClientIdentifier        string
	TopicAliasMaximum               *uint16
	ReasonString                    string
	UserProperty                    []UserProperty
	WildcardSubscriptionAvailable   *byte
	SubscriptionIdentifierAvailable *byte
	SharedSubscriptionAvailable     *byte
	ServerKeepAlive                 *uint16
	ResponseInformation             string
	ServerReference                 string
	AuthenticationMethod            string
	AuthenticationData              []byte
}

type ConnAck struct {
	SessionPresent bool
	ReasonCode     byte
	Properties     *ConnAckProperties
}

func (c *ConnAck) Decode(reader io.Reader) error {
	remainingLength, err := decodeVariableByteInteger(reader)
	if err != nil {
		return fmt.Errorf("error decoding remaining length on connack: %w", err)
	}

	buf := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return fmt.Errorf("error reading connack packet: %w", err)
	}

	c.SessionPresent = buf[0]&0x01 == 1
	c.ReasonCode = buf[1]

	if remainingLength > 2 {
		c.Properties = &ConnAckProperties{}
		propsBuf := bytes.NewBuffer(buf[2:])
		err := c.Properties.decode(propsBuf)
		if err != nil {
			return fmt.Errorf("error decoding connack properties: %w", err)
		}
	}

	return nil
}

func (cp *ConnAckProperties) decode(buf *bytes.Buffer) error {
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
			cp.SessionExpiryInterval = &sessionExpiry
		case PropertyAssignedClientIdentifier:
			assignedClientID, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding assigned client identifier: %w", err)
			}
			cp.AssignedClientIdentifier = assignedClientID
		case PropertyServerKeepAlive:
			serverKeepAlive, err := decodeUint16(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding server keep alive: %w", err)
			}
			cp.ServerKeepAlive = &serverKeepAlive
		case PropertyAuthenticationMethod:
			authMethod, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding authentication method: %w", err)
			}
			cp.AuthenticationMethod = authMethod
		case PropertyAuthenticationData:
			authData, err := decodeBinary(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding authentication data: %w", err)
			}
			cp.AuthenticationData = authData
		case PropertyResponseInformation:
			responseInfo, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding response information: %w", err)
			}
			cp.ResponseInformation = responseInfo
		case PropertyServerReference:
			serverRef, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding server reference: %w", err)
			}
			cp.ServerReference = serverRef
		case PropertyReasonString:
			reasonString, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding reason string: %w", err)
			}
			cp.ReasonString = reasonString
		case PropertyReceiveMaximum:
			receiveMax, err := decodeUint16(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding receive maximum: %w", err)
			}
			cp.ReceiveMaximum = &receiveMax
		case PropertyTopicAliasMaximum:
			topicAliasMax, err := decodeUint16(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding topic alias maximum: %w", err)
			}
			cp.TopicAliasMaximum = &topicAliasMax
		case PropertyMaximumQoS:
			maxQos, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding maximum qos: %w", err)
			}
			cp.MaximumQoS = &maxQos
		case PropertyRetainAvailable:
			retainAvailable, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding retain available: %w", err)
			}
			cp.RetainAvailable = &retainAvailable
		case PropertyUserProperty:
			key, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property key: %w", err)
			}
			value, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property value: %w", err)
			}
			if cp.UserProperty == nil {
				cp.UserProperty = make([]UserProperty, 0)
			}
			cp.UserProperty = append(cp.UserProperty, UserProperty{Key: key, Value: value})
		case PropertyMaximumPacketSize:
			maxPacketSize, err := decodeUint32(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding maximum packet size: %w", err)
			}
			cp.MaximumPacketSize = &maxPacketSize
		case PropertyWildcardSubscriptionAvailable:
			wildcardAvailable, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding wildcard subscription available: %w", err)
			}
			cp.WildcardSubscriptionAvailable = &wildcardAvailable
		case PropertySubscriptionIdentifierAvailable:
			subIDAvailable, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding subscription identifier available: %w", err)
			}
			cp.SubscriptionIdentifierAvailable = &subIDAvailable
		case PropertySharedSubscriptionAvailable:
			sharedSubAvailable, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding shared subscription available: %w", err)
			}
			cp.SharedSubscriptionAvailable = &sharedSubAvailable
		default:
			return fmt.Errorf("unsupported property identifier: 0x%02X", propID)
		}
	}

	return nil
}

func (c *ConnAck) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.Bool("session_present", c.SessionPresent),
		slog.Int("reason_code", int(c.ReasonCode)),
	}

	if c.Properties != nil {
		attrs = append(attrs, slog.Any("properties", c.Properties))
	}

	return slog.GroupValue(attrs...)
}

func (cp *ConnAckProperties) LogValue() slog.Value {
	var attrs []slog.Attr

	if cp.SessionExpiryInterval != nil {
		attrs = append(attrs, slog.Uint64("session_expiry_interval", uint64(*cp.SessionExpiryInterval)))
	}
	if cp.ReceiveMaximum != nil {
		attrs = append(attrs, slog.Int("receive_maximum", int(*cp.ReceiveMaximum)))
	}
	if cp.MaximumQoS != nil {
		attrs = append(attrs, slog.Int("maximum_qos", int(*cp.MaximumQoS)))
	}
	if cp.RetainAvailable != nil {
		attrs = append(attrs, slog.Bool("retain_available", *cp.RetainAvailable == 1))
	}
	if cp.MaximumPacketSize != nil {
		attrs = append(attrs, slog.Uint64("maximum_packet_size", uint64(*cp.MaximumPacketSize)))
	}
	if cp.AssignedClientIdentifier != "" {
		attrs = append(attrs, slog.String("assigned_client_id", cp.AssignedClientIdentifier))
	}
	if cp.TopicAliasMaximum != nil {
		attrs = append(attrs, slog.Int("topic_alias_maximum", int(*cp.TopicAliasMaximum)))
	}
	if cp.ReasonString != "" {
		attrs = append(attrs, slog.String("reason_string", cp.ReasonString))
	}
	if len(cp.UserProperty) > 0 {
		userProps := make([]slog.Attr, len(cp.UserProperty))
		for i, prop := range cp.UserProperty {
			userProps[i] = slog.String(prop.Key, prop.Value)
		}
		attrs = append(attrs, slog.Any("user_properties", slog.GroupValue(userProps...)))
	}
	if cp.WildcardSubscriptionAvailable != nil {
		attrs = append(attrs, slog.Bool("wildcard_subscription_available", *cp.WildcardSubscriptionAvailable == 1))
	}
	if cp.SubscriptionIdentifierAvailable != nil {
		attrs = append(attrs, slog.Bool("subscription_identifier_available", *cp.SubscriptionIdentifierAvailable == 1))
	}
	if cp.SharedSubscriptionAvailable != nil {
		attrs = append(attrs, slog.Bool("shared_subscription_available", *cp.SharedSubscriptionAvailable == 1))
	}
	if cp.ServerKeepAlive != nil {
		attrs = append(attrs, slog.Int("server_keep_alive", int(*cp.ServerKeepAlive)))
	}
	if cp.ResponseInformation != "" {
		attrs = append(attrs, slog.String("response_information", cp.ResponseInformation))
	}
	if cp.ServerReference != "" {
		attrs = append(attrs, slog.String("server_reference", cp.ServerReference))
	}
	if cp.AuthenticationMethod != "" {
		attrs = append(attrs, slog.String("authentication_method", cp.AuthenticationMethod))
	}
	if len(cp.AuthenticationData) > 0 {
		attrs = append(attrs, slog.String("authentication_data", fmt.Sprintf("%x", cp.AuthenticationData)))
	}

	return slog.GroupValue(attrs...)
}
