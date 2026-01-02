package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type SubAckProperties struct {
	ReasonString string
	UserProperty []UserProperty
}

type SubAck struct {
	PacketID    uint16
	Properties  *SubAckProperties
	ReasonCodes []byte
}

func (sa *SubAck) Decode(reader io.Reader) error {
	remainingLength, err := decodeVariableByteInteger(reader)
	if err != nil {
		return fmt.Errorf("error decoding remaining length on suback: %w", err)
	}

	if remainingLength < 3 {
		return fmt.Errorf("invalid suback remaining length: %d", remainingLength)
	}

	buf := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return fmt.Errorf("error reading suback packet: %w", err)
	}

	dataBuf := bytes.NewBuffer(buf)

	sa.PacketID, err = decodeUint16(dataBuf)
	if err != nil {
		return fmt.Errorf("error decoding packet id: %w", err)
	}

	sa.Properties = &SubAckProperties{}
	if err := sa.Properties.decode(dataBuf); err != nil {
		return fmt.Errorf("error decoding suback properties: %w", err)
	}

	sa.ReasonCodes = dataBuf.Bytes()

	return nil
}

func (sp *SubAckProperties) decode(buf *bytes.Buffer) error {
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
			sp.ReasonString, err = decodeString(propBuf)
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
			sp.UserProperty = append(sp.UserProperty, UserProperty{Key: key, Value: value})
		default:
			return fmt.Errorf("unsupported suback property identifier: 0x%02X", propID)
		}
	}

	return nil
}
