package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type AckPacket struct {
	PacketID   uint16
	ReasonCode byte
	Properties *AckProperties
}

func (ap *AckPacket) GetPacketID() uint16 {
	return ap.PacketID
}

func (ap *AckPacket) SetPacketID(packetID uint16) {
	ap.PacketID = packetID
}

func (ap *AckPacket) Decode(reader io.Reader) error {
	remainingLength, err := decodeVariableByteInteger(reader)
	if err != nil {
		return fmt.Errorf("error decoding remaining length on ackPacket: %w", err)
	}

	if remainingLength < 2 {
		return fmt.Errorf("invalid ackPacket remaining length: %d", remainingLength)
	}

	buf := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return fmt.Errorf("error reading ackPacket: %w", err)
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
		ap.Properties = &AckProperties{}
		err := ap.Properties.decode(dataBuf)
		if err != nil {
			return fmt.Errorf("error decoding ackProperties: %w", err)
		}
	}

	return nil
}
