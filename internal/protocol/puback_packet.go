package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type PubAckPacket struct {
	PacketID   uint16
	ReasonCode byte
	Properties *PubAckProperties
}

type PubAckProperties struct {
	ReasonString string
	UserProperty []UserProperty
}

func (ap *PubAckPacket) GetPacketID() uint16 {
	return ap.PacketID
}

func (ap *PubAckPacket) SetPacketID(packetID uint16) {
	ap.PacketID = packetID
}

func (ap *PubAckPacket) Decode(reader io.Reader) error {
	remainingLength, err := decodeVariableByteInteger(reader)
	if err != nil {
		return fmt.Errorf("error decoding remaining length on pubAckPacket: %w", err)
	}

	if remainingLength < 2 {
		return fmt.Errorf("invalid pubAckPacket remaining length: %d", remainingLength)
	}

	buf := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return fmt.Errorf("error reading pubAckPacket: %w", err)
	}

	dataBuf := bytes.NewBuffer(buf)

	ap.PacketID, err = decodeUint16(dataBuf)
	if err != nil {
		return fmt.Errorf("error decoding packet id: %w", err)
	}

	if remainingLength > 2 {
		ap.ReasonCode, err = dataBuf.ReadByte()
		if err != nil {
			return fmt.Errorf("error decoding reason code: %w", err)
		}
	}

	if remainingLength > 3 {
		ap.Properties = &PubAckProperties{}
		err := ap.Properties.decode(dataBuf)
		if err != nil {
			return fmt.Errorf("error decoding pubAckProperties: %w", err)
		}
	}

	return nil
}

func (ap *PubAckProperties) Encode() ([]byte, error) {
	var propsBuf bytes.Buffer

	if ap.ReasonString != "" {
		propsBuf.WriteByte(PropertyReasonString)
		err := encodeString(&propsBuf, ap.ReasonString)
		if err != nil {
			return nil, err
		}
	}

	for _, userProp := range ap.UserProperty {
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

func (ap *PubAckProperties) decode(buf *bytes.Buffer) error {
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
		case PropertyReasonString:
			reasonString, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding reason string: %w", err)
			}
			ap.ReasonString = reasonString

		case PropertyUserProperty:
			key, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property key: %w", err)
			}
			value, err := decodeString(propBuf)
			if err != nil {
				return fmt.Errorf("error decoding user property value: %w", err)
			}
			if ap.UserProperty == nil {
				ap.UserProperty = make([]UserProperty, 0)
			}
			ap.UserProperty = append(ap.UserProperty, UserProperty{Key: key, Value: value})
		default:
			return fmt.Errorf("unsupported property identifier: 0x%02X", propID)
		}
	}

	return nil
}
