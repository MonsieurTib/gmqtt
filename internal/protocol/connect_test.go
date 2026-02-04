package protocol

import (
	"testing"
)

func TestConnectValidateUTF8(t *testing.T) {
	tests := []struct {
		name        string
		willMessage string
		indicator   *bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid UTF-8 will message with indicator true",
			willMessage: "Hello, World!",
			indicator:   boolPtr(true),
			wantErr:     false,
		},
		{
			name:        "invalid UTF-8 will message with indicator true",
			willMessage: string([]byte{0xFF, 0xFE, 0xFD}),
			indicator:   boolPtr(true),
			wantErr:     true,
			errContains: "invalid will message",
		},
		{
			name:        "binary will message with indicator false",
			willMessage: string([]byte{0x00, 0x01, 0x02, 0xFF}),
			indicator:   boolPtr(false),
			wantErr:     false,
		},
		{
			name:        "binary will message with indicator nil",
			willMessage: string([]byte{0x00, 0x01, 0x02, 0xFF}),
			indicator:   nil,
			wantErr:     false,
		},
		{
			name:        "empty will message with indicator true",
			willMessage: "",
			indicator:   boolPtr(true),
			wantErr:     false,
		},
		{
			name:        "unicode will message with indicator true",
			willMessage: "Hello, 世界! 🌍",
			indicator:   boolPtr(true),
			wantErr:     false,
		},
		{
			name:        "no will message - should pass",
			willMessage: "",
			indicator:   nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var willProperties *WillProperties
			if tt.indicator != nil {
				willProperties = &WillProperties{
					PayloadFormatIndicator: tt.indicator,
				}
			}

			opt := ConnectOptions{
				ClientID:       "test-client",
				WillTopic:      "will/topic",
				WillMessage:    tt.willMessage,
				WillProperties: willProperties,
			}

			connect := NewConnect(opt)
			err := connect.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error but got none")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Validate() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}
