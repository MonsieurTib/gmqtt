package protocol

import (
	"bytes"
	"fmt"
	"io"
	"net"
)

type PubAck struct {
	PacketID   uint16
	ReasonCode byte
	Properties *PubAckProperties
}

type PubAckProperties struct {
	ReasonString string
	UserProperty []UserProperty
}

func (p *PubAck) Encode() (net.Buffers, error) {
	return nil, nil
}

func (p *PubAck) Decode(reader io.Reader) error {
	remainingLength, err := decodeVariableByteInteger(reader)
	if err != nil {
		return fmt.Errorf("error decoding remaining length on puback: %w", err)
	}

	if remainingLength < 2 {
		return fmt.Errorf("invalid puback remaining length: %d", remainingLength)
	}

	buf := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return fmt.Errorf("error reading puback packet: %w", err)
	}

	dataBuf := bytes.NewBuffer(buf)

	p.PacketID, err = decodeUint16(dataBuf)
	if err != nil {
		return fmt.Errorf("error decoding packet id: %w", err)
	}

	if remainingLength > 2 {
		p.ReasonCode, err = dataBuf.ReadByte()
		if err != nil {
			return fmt.Errorf("error decoding reason code: %w", err)
		}
	}

	if remainingLength > 3 {
		p.Properties = &PubAckProperties{}
		err := p.Properties.decode(dataBuf)
		if err != nil {
			return fmt.Errorf("error decoding puback properties: %w", err)
		}
	}

	return nil
}

func (pp *PubAckProperties) decode(buf *bytes.Buffer) error {
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
			pp.ReasonString = reasonString

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
		default:
			return fmt.Errorf("unsupported property identifier: 0x%02X", propID)
		}
	}

	return nil
}
