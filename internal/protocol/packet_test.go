package protocol

import (
	"bytes"
	"testing"
)

func Test_encodeVariableByteInteger(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   []byte
	}{
		{
			name:   "zero",
			length: 0,
			want:   []byte{0x00},
		},
		{
			name:   "single byte",
			length: 127,
			want:   []byte{0x7F},
		},
		{
			name:   "two bytes",
			length: 128,
			want:   []byte{0x80, 0x01},
		},
		{
			name:   "two bytes max",
			length: 16383,
			want:   []byte{0xFF, 0x7F},
		},
		{
			name:   "three bytes",
			length: 16384,
			want:   []byte{0x80, 0x80, 0x01},
		},
		{
			name:   "four bytes max",
			length: 268435455,
			want:   []byte{0xFF, 0xFF, 0xFF, 0x7F},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			encodeVariableByteInteger(&buf, tt.length)
			if got := buf.Bytes(); !bytes.Equal(got, tt.want) {
				t.Errorf("encodeVariableByteInteger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_decodeVariableByteInteger(t *testing.T) {
	tests := []struct {
		name    string
		buf     []byte
		want    int
		wantErr bool
	}{
		{
			name: "zero",
			buf:  []byte{0x00},
			want: 0,
		},
		{
			name: "single byte",
			buf:  []byte{0x7F},
			want: 127,
		},
		{
			name: "two bytes",
			buf:  []byte{0x80, 0x01},
			want: 128,
		},
		{
			name: "two bytes max",
			buf:  []byte{0xFF, 0x7F},
			want: 16383,
		},
		{
			name: "three bytes",
			buf:  []byte{0x80, 0x80, 0x01},
			want: 16384,
		},
		{
			name: "four bytes max",
			buf:  []byte{0xFF, 0xFF, 0xFF, 0x7F},
			want: 268435455,
		},
		{
			name:    "malformed (too long)",
			buf:     []byte{0x80, 0x80, 0x80, 0x80, 0x01},
			wantErr: true,
		},
		{
			name:    "incomplete",
			buf:     []byte{0x80},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeVariableByteInteger(bytes.NewBuffer(tt.buf))
			if err != nil {
				if !tt.wantErr {
					t.Errorf("decodeVariableByteInteger() unexpected error: %v", err)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("decodeVariableByteInteger() expected an error but got none")
			}
			if got != tt.want {
				t.Errorf("decodeVariableByteInteger() = %v, want %v", got, tt.want)
			}
		})
	}
}
