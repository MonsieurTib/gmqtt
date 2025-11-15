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

func (p *ConnAck) Decode(reader io.Reader) error {
	remainingLength, err := decodeVariableByteInteger(reader)
	if err != nil {
		return fmt.Errorf("error decoding remaining length on connack: %w", err)
	}

	buf := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return fmt.Errorf("error reading connack packet: %w", err)
	}

	p.SessionPresent = buf[0]&0x01 == 1
	p.ReasonCode = buf[1]

	if remainingLength > 2 {
		p.Properties = &ConnAckProperties{}
		propsBuf := bytes.NewBuffer(buf[2:])
		err := p.Properties.decode(propsBuf)
		if err != nil {
			return fmt.Errorf("error decoding connack properties: %w", err)
		}
	}

	return nil
}

func (pp *ConnAckProperties) decode(buf *bytes.Buffer) error {
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
			pp.SessionExpiryInterval = &sessionExpiry
		case PropertyAssignedClientIdentifier:
			assignedClientID, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding assigned client identifier: %w", err)
			}
			pp.AssignedClientIdentifier = assignedClientID
		case PropertyServerKeepAlive:
			serverKeepAlive, err := decodeUint16(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding server keep alive: %w", err)
			}
			pp.ServerKeepAlive = &serverKeepAlive
		case PropertyAuthenticationMethod:
			authMethod, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding authentication method: %w", err)
			}
			pp.AuthenticationMethod = authMethod
		case PropertyAuthenticationData:
			authData, err := decodeBinary(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding authentication data: %w", err)
			}
			pp.AuthenticationData = authData
		case PropertyResponseInformation:
			responseInfo, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding response information: %w", err)
			}
			pp.ResponseInformation = responseInfo
		case PropertyServerReference:
			serverRef, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding server reference: %w", err)
			}
			pp.ServerReference = serverRef
		case PropertyReasonString:
			reasonString, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding reason string: %w", err)
			}
			pp.ReasonString = reasonString
		case PropertyReceiveMaximum:
			receiveMax, err := decodeUint16(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding receive maximum: %w", err)
			}
			pp.ReceiveMaximum = &receiveMax
		case PropertyTopicAliasMaximum:
			topicAliasMax, err := decodeUint16(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding topic alias maximum: %w", err)
			}
			pp.TopicAliasMaximum = &topicAliasMax
		case PropertyMaximumQoS:
			maxQos, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding maximum qos: %w", err)
			}
			pp.MaximumQoS = &maxQos
		case PropertyRetainAvailable:
			retainAvailable, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding retain available: %w", err)
			}
			pp.RetainAvailable = &retainAvailable
		case PropertyUserProperty:
			key, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property key: %w", err)
			}
			value, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property value: %w", err)
			}
			if pp.UserProperty == nil {
				pp.UserProperty = make([]UserProperty, 0)
			}
			pp.UserProperty = append(pp.UserProperty, UserProperty{Key: key, Value: value})
		case PropertyMaximumPacketSize:
			maxPacketSize, err := decodeUint32(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding maximum packet size: %w", err)
			}
			pp.MaximumPacketSize = &maxPacketSize
		case PropertyWildcardSubscriptionAvailable:
			wildcardAvailable, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding wildcard subscription available: %w", err)
			}
			pp.WildcardSubscriptionAvailable = &wildcardAvailable
		case PropertySubscriptionIdentifierAvailable:
			subIDAvailable, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding subscription identifier available: %w", err)
			}
			pp.SubscriptionIdentifierAvailable = &subIDAvailable
		case PropertySharedSubscriptionAvailable:
			sharedSubAvailable, err := propBuf.ReadByte()
			if err != nil {
				return fmt.Errorf("error decoding shared subscription available: %w", err)
			}
			pp.SharedSubscriptionAvailable = &sharedSubAvailable
		default:
			return fmt.Errorf("unsupported property identifier: 0x%02X", propID)
		}
	}

	return nil
}

func (p *ConnAck) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.Bool("session_present", p.SessionPresent),
		slog.Int("reason_code", int(p.ReasonCode)),
	}

	if p.Properties != nil {
		attrs = append(attrs, slog.Any("properties", p.Properties))
	}

	return slog.GroupValue(attrs...)
}

func (pp *ConnAckProperties) LogValue() slog.Value {
	var attrs []slog.Attr

	if pp.SessionExpiryInterval != nil {
		attrs = append(
			attrs,
			slog.Uint64("session_expiry_interval", uint64(*pp.SessionExpiryInterval)),
		)
	}
	if pp.ReceiveMaximum != nil {
		attrs = append(attrs, slog.Int("receive_maximum", int(*pp.ReceiveMaximum)))
	}
	if pp.MaximumQoS != nil {
		attrs = append(attrs, slog.Int("maximum_qos", int(*pp.MaximumQoS)))
	}
	if pp.RetainAvailable != nil {
		attrs = append(attrs, slog.Bool("retain_available", *pp.RetainAvailable == 1))
	}
	if pp.MaximumPacketSize != nil {
		attrs = append(attrs, slog.Uint64("maximum_packet_size", uint64(*pp.MaximumPacketSize)))
	}
	if pp.AssignedClientIdentifier != "" {
		attrs = append(attrs, slog.String("assigned_client_id", pp.AssignedClientIdentifier))
	}
	if pp.TopicAliasMaximum != nil {
		attrs = append(attrs, slog.Int("topic_alias_maximum", int(*pp.TopicAliasMaximum)))
	}
	if pp.ReasonString != "" {
		attrs = append(attrs, slog.String("reason_string", pp.ReasonString))
	}
	if len(pp.UserProperty) > 0 {
		userProps := make([]slog.Attr, len(pp.UserProperty))
		for i, prop := range pp.UserProperty {
			userProps[i] = slog.String(prop.Key, prop.Value)
		}
		attrs = append(attrs, slog.Any("user_properties", slog.GroupValue(userProps...)))
	}
	if pp.WildcardSubscriptionAvailable != nil {
		attrs = append(
			attrs,
			slog.Bool("wildcard_subscription_available", *pp.WildcardSubscriptionAvailable == 1),
		)
	}
	if pp.SubscriptionIdentifierAvailable != nil {
		attrs = append(
			attrs,
			slog.Bool(
				"subscription_identifier_available",
				*pp.SubscriptionIdentifierAvailable == 1,
			),
		)
	}
	if pp.SharedSubscriptionAvailable != nil {
		attrs = append(
			attrs,
			slog.Bool("shared_subscription_available", *pp.SharedSubscriptionAvailable == 1),
		)
	}
	if pp.ServerKeepAlive != nil {
		attrs = append(attrs, slog.Int("server_keep_alive", int(*pp.ServerKeepAlive)))
	}
	if pp.ResponseInformation != "" {
		attrs = append(attrs, slog.String("response_information", pp.ResponseInformation))
	}
	if pp.ServerReference != "" {
		attrs = append(attrs, slog.String("server_reference", pp.ServerReference))
	}
	if pp.AuthenticationMethod != "" {
		attrs = append(attrs, slog.String("authentication_method", pp.AuthenticationMethod))
	}
	if len(pp.AuthenticationData) > 0 {
		attrs = append(
			attrs,
			slog.String("authentication_data", fmt.Sprintf("%x", pp.AuthenticationData)),
		)
	}

	return slog.GroupValue(attrs...)
}
