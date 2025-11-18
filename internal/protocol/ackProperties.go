package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type AckProperties struct {
	ReasonString string
	UserProperty []UserProperty
}

func (ap *AckProperties) Encode() ([]byte, error) {
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

func (ap *AckProperties) decode(buf *bytes.Buffer) error {
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
