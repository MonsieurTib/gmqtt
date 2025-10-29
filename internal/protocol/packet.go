package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	_ byte = iota
	TypeConnect
	TypeConnack
	TypePublish
	TypePubAck
	TypePubRec
	TypePubRel
	TypePubComp
	TypeSubscribe
	TypeSubAck
	TypeUnsubscribe
	TypeUnSubAck
	TypePingReq
	TypePingResp
	TypeDisconnect
	TypeAuth
)

const (
	PropertyPayloadFormatIndicator          = 0x01
	PropertyMessageExpiryInterval           = 0x02
	PropertyContentType                     = 0x03
	PropertyResponseTopic                   = 0x08
	PropertyCorrelationData                 = 0x09
	PropertySubscriptionIdentifier          = 0x0B
	PropertySessionExpiryInterval           = 0x11
	PropertyAssignedClientIdentifier        = 0x12
	PropertyServerKeepAlive                 = 0x13
	PropertyAuthenticationMethod            = 0x15
	PropertyAuthenticationData              = 0x16
	PropertyRequestProblemInfo              = 0x17
	PropertyWillDelayInterval               = 0x18
	PropertyRequestResponseInfo             = 0x19
	PropertyResponseInformation             = 0x1A
	PropertyServerReference                 = 0x1C
	PropertyReasonString                    = 0x1F
	PropertyReceiveMaximum                  = 0x21
	PropertyTopicAliasMaximum               = 0x22
	PropertyTopicAlias                      = 0x23
	PropertyMaximumQoS                      = 0x24
	PropertyRetainAvailable                 = 0x25
	PropertyUserProperty                    = 0x26
	PropertyMaximumPacketSize               = 0x27
	PropertyWildcardSubscriptionAvailable   = 0x28
	PropertySubscriptionIdentifierAvailable = 0x29
	PropertySharedSubscriptionAvailable     = 0x2A
)

type UserProperty struct {
	Key, Value string
}

type Packet interface {
	Encode() ([]byte, error)
}

// from https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html#_Toc3901011
// The Variable Byte Integer is encoded using an encoding scheme which uses a single byte for values up to 127.
// Larger values are handled as follows. The least significant seven bits of each byte encode the data, and the most
// significant bit is used to indicate whether there are bytes following in the representation.
// Thus, each byte encodes 128 values and a "continuation bit".
// The maximum number of bytes in the Variable Byte Integer field is four.
// The encoded value MUST use the minimum number of bytes necessary to represent the value
func encodeVariableByteInteger(buf *bytes.Buffer, length int) {
	for {
		encodedByte := byte(length % 128)
		length /= 128
		if length > 0 {
			encodedByte |= 0x80
		}
		buf.WriteByte(encodedByte)
		if length <= 0 {
			break
		}
	}
}

func decodeVariableByteInteger(r io.Reader) (int, error) {
	value := 0
	multiplier := 1
	buf := make([]byte, 1)

	for {
		_, err := r.Read(buf)
		if err != nil {
			return 0, err
		}
		value += int(buf[0]&0x7F) * multiplier
		if multiplier > 128*128*128 {
			return 0, errors.New("malformed Variable Byte Integer (too long)")
		}
		if (buf[0] & 128) == 0 {
			break
		}
		multiplier *= 128
	}

	return value, nil
}

func decodeVariableByteIntegerFromBuffer(buf *bytes.Buffer) (int, error) {
	value := 0
	multiplier := 1

	for {
		b, err := buf.ReadByte()
		if err != nil {
			return 0, err
		}
		value += int(b&0x7F) * multiplier
		if multiplier > 128*128*128 {
			return 0, errors.New("malformed Variable Byte Integer (too long)")
		}
		if (b & 128) == 0 {
			break
		}
		multiplier *= 128
	}

	return value, nil
}

// from https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html#_Toc3901010
// string data is prefixed with a two byte length field that gives the number of bytes in the UTF-8 encoded string itself.
// Consequently, the maximum size of a UTF-8 Encoded String is 65,535 bytes
// Example: given the string "hello", the encoded string will be 0x00 0x05 0x68 0x65 0x6c 0x6c 0x6f
// The first two bytes 0x00 0x05 represent the length of the string "hello" and the remaining bytes are the UTF-8 encoded string
// Another example: give a 300 bytes string, the encoded string will be 0x01 0x2C

func encodeString(buf *bytes.Buffer, str string) error {
	err := encodeUint16(buf, uint16(len(str)))
	if err != nil {
		return err
	}

	buf.WriteString(str)
	return nil
}

func encodeBinary(buf *bytes.Buffer, data []byte) error {
	//	binary.Write(buffer, binary.BigEndian, uint16(len(data)))
	err := encodeUint16(buf, uint16(len(data)))
	if err != nil {
		return err
	}
	buf.Write(data)
	return nil
}

func encodeUint16(buff *bytes.Buffer, n uint16) error {
	_, err := buff.Write([]byte{byte(n >> 8), byte(n)})
	return err
}

func encodeUint32(buff *bytes.Buffer, n uint32) error {
	_, err := buff.Write([]byte{
		byte(n >> 24),
		byte(n >> 16),
		byte(n >> 8),
		byte(n),
	})
	return err
}

func decodeUint16(buf *bytes.Buffer) (uint16, error) {
	b := buf.Next(2)
	if len(b) < 2 {
		return 0, fmt.Errorf("not enough bytes")
	}
	return uint16(b[0])<<8 | uint16(b[1]), nil
}

func decodeUint32(buf *bytes.Buffer) (uint32, error) {
	b := buf.Next(4)
	if len(b) < 4 {
		return 0, fmt.Errorf("not enough bytes")
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

func decodeBinary(buf *bytes.Buffer) ([]byte, error) {
	l, err := decodeUint16(buf)
	if err != nil {
		return nil, err
	}

	if l == 0 {
		return []byte{}, nil
	}

	result := make([]byte, l)

	_, err = buf.Read(result)
	if err != nil {
		return nil, fmt.Errorf("failed to read binary data: %w", err)
	}

	return result, nil
}

func decodeString(buf *bytes.Buffer) (string, error) {
	b, err := decodeBinary(buf)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
