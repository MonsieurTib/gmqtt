package protocol

import (
	"testing"
)

func TestNewPublishUTF8Validation(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		indicator   *bool
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid UTF-8 with indicator true",
			payload:   []byte("Hello, World!"),
			indicator: boolPtr(true),
			wantErr:   false,
		},
		{
			name:        "invalid UTF-8 with indicator true",
			payload:     []byte{0xFF, 0xFE, 0xFD},
			indicator:   boolPtr(true),
			wantErr:     true,
			errContains: "invalid payload",
		},
		{
			name:      "binary data with indicator false",
			payload:   []byte{0x00, 0x01, 0x02, 0xFF},
			indicator: boolPtr(false),
			wantErr:   false,
		},
		{
			name:      "binary data with indicator nil",
			payload:   []byte{0x00, 0x01, 0x02, 0xFF},
			indicator: nil,
			wantErr:   false,
		},
		{
			name:      "empty payload with indicator true",
			payload:   []byte{},
			indicator: boolPtr(true),
			wantErr:   false,
		},
		{
			name:      "unicode characters with indicator true",
			payload:   []byte("Hello, 世界! 🌍"),
			indicator: boolPtr(true),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := PublishOptions{
				Topic:   "test/topic",
				Payload: tt.payload,
				PublishProperties: &PublishProperties{
					PayloadFormatIndicator: tt.indicator,
				},
			}

			_, err := NewPublish(opt)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewPublish() expected error but got none")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewPublish() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("NewPublish() unexpected error = %v", err)
				}
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
