package protocol

import (
	"bytes"
	"fmt"
	"log/slog"
)

const (
	FlagCleanStart   = 0x02
	FlagWillFlag     = 0x04
	FlagWillRetain   = 0x20
	FlagPassword     = 0x40
	FlagUsername     = 0x80
	FlagWillQoSShift = 3
)

type Connect struct {
	protocolVersion byte
	protocolName    string
	cleanStart      bool
	keepAlive       uint16
	clientID        string
	properties      *ConnectProperties
	username        string
	password        []byte
	willFlag        bool
	willTopic       string
	willMessage     string
	willQoS         byte
	willRetain      bool
	willProperties  *WillProperties
}

type ConnectProperties struct {
	SessionExpiryInterval      *uint32
	ReceiveMaximum             *uint16
	MaximumPacketSize          *uint32
	TopicAliasMaximum          *uint16
	RequestResponseInformation *byte
	RequestProblemInformation  *byte
	UserProperty               []UserProperty
	AuthenticationMethod       string
	AuthenticationData         []byte
}

type ConnectOptions struct {
	CleanStart        bool
	KeepAlive         uint16
	ConnectProperties *ConnectProperties
	ClientID          string
	WillTopic         string
	WillMessage       string
	WillQoS           byte
	WillRetain        bool
	WillProperties    *WillProperties
	Username          string
	Password          string
}

type WillProperties struct {
	WillDelayInterval      *uint32
	PayloadFormatIndicator *bool
	MessageExpiryInterval  *uint32
	ContentType            string
	ResponseTopic          string
	CorrelationData        []byte
	UserProperty           []UserProperty
}

func NewConnect(opt ConnectOptions) *Connect {
	c := &Connect{
		protocolVersion: 0x05,
		protocolName:    "MQTT",
		cleanStart:      opt.CleanStart,
		keepAlive:       opt.KeepAlive,
		clientID:        opt.ClientID,
		properties:      opt.ConnectProperties,
		username:        opt.Username,
		password:        []byte(opt.Password),
	}

	if opt.WillTopic != "" && opt.WillMessage != "" {
		c.willFlag = true
		c.willTopic = opt.WillTopic
		c.willMessage = opt.WillMessage
		c.willQoS = opt.WillQoS
		c.willRetain = opt.WillRetain
		c.willProperties = opt.WillProperties
	}

	return c
}

func (c *Connect) Encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(TypeConnect << 4)

	header, err := c.encodeVariableHeader()
	if err != nil {
		return nil, err
	}

	payload, err := c.encodePayload()
	if err != nil {
		return nil, err
	}

	remainingLength := len(payload) + len(header)
	encodeVariableByteInteger(&buf, remainingLength)
	buf.Write(header)
	buf.Write(payload)

	return buf.Bytes(), nil
}

func (c *Connect) encodeConnectFlags() byte {
	var flags byte

	if c.cleanStart {
		flags |= FlagCleanStart
	}
	if c.willFlag {
		flags |= FlagWillFlag
		flags |= c.willQoS << FlagWillQoSShift
		if c.willRetain {
			flags |= FlagWillRetain
		}
	}
	if c.username != "" {
		flags |= FlagUsername
	}
	if len(c.password) > 0 {
		flags |= FlagPassword
	}

	return flags
}

func (c *Connect) encodeVariableHeader() ([]byte, error) {
	var buf bytes.Buffer

	if err := encodeString(&buf, c.protocolName); err != nil {
		return nil, err
	}

	buf.WriteByte(c.protocolVersion)
	buf.WriteByte(c.encodeConnectFlags())
	err := encodeUint16(&buf, c.keepAlive)
	if err != nil {
		return nil, err
	}

	if c.properties == nil {
		buf.WriteByte(0x00)
	} else {
		propBytes, err := c.properties.Encode()
		if err != nil {
			return nil, err
		}
		buf.Write(propBytes)
	}

	return buf.Bytes(), nil
}

func (c *Connect) encodePayload() ([]byte, error) {
	var buf bytes.Buffer

	if err := encodeString(&buf, c.clientID); err != nil {
		return nil, err
	}

	if c.willFlag {
		if c.willProperties == nil {
			buf.WriteByte(0x00)
		} else {
			propBytes, err := c.willProperties.Encode()
			if err != nil {
				return nil, err
			}
			buf.Write(propBytes)
		}
		if err := encodeString(&buf, c.willTopic); err != nil {
			return nil, err
		}
		if err := encodeBinary(&buf, []byte(c.willMessage)); err != nil {
			return nil, err
		}
	}

	if c.username != "" {
		if err := encodeString(&buf, c.username); err != nil {
			return nil, err
		}
	}

	if len(c.password) > 0 {
		if err := encodeBinary(&buf, c.password); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (cp *ConnectProperties) Encode() ([]byte, error) {
	var propsBuf bytes.Buffer

	if cp.SessionExpiryInterval != nil {
		propsBuf.WriteByte(PropertySessionExpiryInterval)
		err := encodeUint32(&propsBuf, *cp.SessionExpiryInterval)
		if err != nil {
			return nil, err
		}
	}

	if cp.ReceiveMaximum != nil {
		propsBuf.WriteByte(PropertyReceiveMaximum)
		err := encodeUint16(&propsBuf, *cp.ReceiveMaximum)
		if err != nil {
			return nil, err
		}
	}

	if cp.MaximumPacketSize != nil {
		propsBuf.WriteByte(PropertyMaximumPacketSize)
		err := encodeUint32(&propsBuf, *cp.MaximumPacketSize)
		if err != nil {
			return nil, err
		}
	}

	if cp.TopicAliasMaximum != nil {
		propsBuf.WriteByte(PropertyTopicAliasMaximum)
		err := encodeUint16(&propsBuf, *cp.TopicAliasMaximum)
		if err != nil {
			return nil, err
		}
	}

	if cp.RequestResponseInformation != nil {
		propsBuf.WriteByte(PropertyRequestResponseInfo)
		propsBuf.WriteByte(*cp.RequestResponseInformation)
	}

	if cp.RequestProblemInformation != nil {
		propsBuf.WriteByte(PropertyRequestProblemInfo)
		propsBuf.WriteByte(*cp.RequestProblemInformation)
	}

	for _, userProp := range cp.UserProperty {
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

	if cp.AuthenticationMethod != "" {
		propsBuf.WriteByte(PropertyAuthenticationMethod)
		err := encodeString(&propsBuf, cp.AuthenticationMethod)
		if err != nil {
			return nil, err
		}
	}

	if len(cp.AuthenticationData) > 0 {
		propsBuf.WriteByte(PropertyAuthenticationData)
		err := encodeBinary(&propsBuf, cp.AuthenticationData)
		if err != nil {
			return nil, err
		}
	}

	var finalBuf bytes.Buffer
	encodeVariableByteInteger(&finalBuf, propsBuf.Len())
	finalBuf.Write(propsBuf.Bytes())

	return finalBuf.Bytes(), nil
}

func (wp *WillProperties) Encode() ([]byte, error) {
	var propsBuf bytes.Buffer

	if wp.WillDelayInterval != nil {
		propsBuf.WriteByte(PropertyWillDelayInterval)
		err := encodeUint32(&propsBuf, *wp.WillDelayInterval)
		if err != nil {
			return nil, err
		}
	}

	if wp.PayloadFormatIndicator != nil && *wp.PayloadFormatIndicator {
		propsBuf.WriteByte(PropertyPayloadFormatIndicator)
		propsBuf.WriteByte(0x01)
	}

	if wp.MessageExpiryInterval != nil {
		propsBuf.WriteByte(PropertyMessageExpiryInterval)
		err := encodeUint32(&propsBuf, *wp.MessageExpiryInterval)
		if err != nil {
			return nil, err
		}
	}

	if wp.ContentType != "" {
		propsBuf.WriteByte(PropertyContentType)
		err := encodeString(&propsBuf, wp.ContentType)
		if err != nil {
			return nil, err
		}
	}

	if wp.ResponseTopic != "" {
		propsBuf.WriteByte(PropertyResponseTopic)
		err := encodeString(&propsBuf, wp.ResponseTopic)
		if err != nil {
			return nil, err
		}
	}

	if len(wp.CorrelationData) > 0 {
		propsBuf.WriteByte(PropertyCorrelationData)
		err := encodeBinary(&propsBuf, wp.CorrelationData)
		if err != nil {
			return nil, err
		}
	}

	for _, userProp := range wp.UserProperty {
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

func (c *Connect) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("protocol", c.protocolName),
		slog.Int("version", int(c.protocolVersion)),
		slog.String("client_id", c.clientID),
		slog.Bool("clean_start", c.cleanStart),
		slog.Int("keep_alive", int(c.keepAlive)),
	}

	if c.username != "" {
		attrs = append(attrs, slog.String("username", c.username))
	}

	if c.willFlag {
		willAttrs := []slog.Attr{
			slog.String("topic", c.willTopic),
			slog.Int("qos", int(c.willQoS)),
			slog.Bool("retain", c.willRetain),
		}
		if c.willProperties != nil {
			willAttrs = append(willAttrs, slog.Any("properties", c.willProperties))
		}
		attrs = append(attrs, slog.Any("will", slog.GroupValue(willAttrs...)))
	}

	if c.properties != nil {
		attrs = append(attrs, slog.Any("properties", c.properties))
	}

	return slog.GroupValue(attrs...)
}

func (cp *ConnectProperties) LogValue() slog.Value {
	var attrs []slog.Attr

	if cp.SessionExpiryInterval != nil {
		attrs = append(
			attrs,
			slog.Uint64("session_expiry_interval", uint64(*cp.SessionExpiryInterval)),
		)
	}
	if cp.ReceiveMaximum != nil {
		attrs = append(attrs, slog.Int("receive_maximum", int(*cp.ReceiveMaximum)))
	}
	if cp.MaximumPacketSize != nil {
		attrs = append(attrs, slog.Uint64("maximum_packet_size", uint64(*cp.MaximumPacketSize)))
	}
	if cp.TopicAliasMaximum != nil {
		attrs = append(attrs, slog.Int("topic_alias_maximum", int(*cp.TopicAliasMaximum)))
	}
	if cp.RequestResponseInformation != nil {
		attrs = append(
			attrs,
			slog.Int("request_response_info", int(*cp.RequestResponseInformation)),
		)
	}
	if cp.RequestProblemInformation != nil {
		attrs = append(attrs, slog.Int("request_problem_info", int(*cp.RequestProblemInformation)))
	}
	if len(cp.UserProperty) > 0 {
		userProps := make([]slog.Attr, len(cp.UserProperty))
		for i, prop := range cp.UserProperty {
			userProps[i] = slog.String(prop.Key, prop.Value)
		}
		attrs = append(attrs, slog.Any("user_properties", slog.GroupValue(userProps...)))
	}
	if cp.AuthenticationMethod != "" {
		attrs = append(attrs, slog.String("authentication_method", cp.AuthenticationMethod))
	}
	if len(cp.AuthenticationData) > 0 {
		attrs = append(
			attrs,
			slog.String("authentication_data", fmt.Sprintf("%x", cp.AuthenticationData)),
		)
	}

	return slog.GroupValue(attrs...)
}

func (wp *WillProperties) LogValue() slog.Value {
	var attrs []slog.Attr

	if wp.WillDelayInterval != nil {
		attrs = append(attrs, slog.Uint64("will_delay_interval", uint64(*wp.WillDelayInterval)))
	}
	if wp.PayloadFormatIndicator != nil {
		attrs = append(attrs, slog.Bool("payload_format_utf8", *wp.PayloadFormatIndicator))
	}
	if wp.MessageExpiryInterval != nil {
		attrs = append(
			attrs,
			slog.Uint64("message_expiry_interval", uint64(*wp.MessageExpiryInterval)),
		)
	}
	if wp.ContentType != "" {
		attrs = append(attrs, slog.String("content_type", wp.ContentType))
	}
	if wp.ResponseTopic != "" {
		attrs = append(attrs, slog.String("response_topic", wp.ResponseTopic))
	}
	if len(wp.CorrelationData) > 0 {
		attrs = append(
			attrs,
			slog.String("correlation_data", fmt.Sprintf("%x", wp.CorrelationData)),
		)
	}
	if len(wp.UserProperty) > 0 {
		userProps := make([]slog.Attr, len(wp.UserProperty))
		for i, prop := range wp.UserProperty {
			userProps[i] = slog.String(prop.Key, prop.Value)
		}
		attrs = append(attrs, slog.Any("user_properties", slog.GroupValue(userProps...)))
	}

	return slog.GroupValue(attrs...)
}
