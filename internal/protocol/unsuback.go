package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type UnSubAckProperties struct {
	ReasonString string
	UserProperty []UserProperty
}

type UnSubAck struct {
	PacketID    uint16
	Properties  *UnSubAckProperties
	ReasonCodes []byte
}

func (ua *UnSubAck) Decode(reader io.Reader) error {
	remainingLength, err := decodeVariableByteInteger(reader)
	if err != nil {
		return fmt.Errorf("error decoding remaining length on unsuback: %w", err)
	}

	if remainingLength < 3 {
		return fmt.Errorf("invalid unsuback remaining length: %d", remainingLength)
	}

	buf := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return fmt.Errorf("error reading unsuback packet: %w", err)
	}

	dataBuf := bytes.NewBuffer(buf)

	ua.PacketID, err = decodeUint16(dataBuf)
	if err != nil {
		return fmt.Errorf("error decoding packet id: %w", err)
	}

	ua.Properties = &UnSubAckProperties{}
	if err := ua.Properties.decode(dataBuf); err != nil {
		return fmt.Errorf("error decoding unsuback properties: %w", err)
	}

	ua.ReasonCodes = dataBuf.Bytes()

	return nil
}

func (up *UnSubAckProperties) decode(buf *bytes.Buffer) error {
	propLen, err := decodeVariableByteIntegerFromBuffer(buf)
	if err != nil {
		return fmt.Errorf("error decoding properties length: %w", err)
	}

	if propLen == 0 {
		return nil
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
		case PropertyReasonString:
			up.ReasonString, err = decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding reason string: %w", err)
			}
		case PropertyUserProperty:
			key, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property key: %w", err)
			}
			value, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property value: %w", err)
			}
			up.UserProperty = append(up.UserProperty, UserProperty{Key: key, Value: value})
		default:
			return fmt.Errorf("unsupported unsuback property identifier: 0x%02X", propID)
		}
	}

	return nil
}
