package protocol

import (
	"fmt"
	"io"
)

type PingRes struct{}

func (p *PingRes) Decode(reader io.Reader) error {
	remainingLength, err := decodeVariableByteInteger(reader)
	if err != nil {
		return fmt.Errorf("error decoding remaining length on pingResp: %w", err)
	}

	buf := make([]byte, remainingLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return fmt.Errorf("error reading pingresp packet: %w", err)
	}

	return nil
}
