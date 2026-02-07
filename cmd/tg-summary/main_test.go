package main

import "testing"

func TestNormalizeChatID(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected int64
	}{
		{
			name:     "raw channel id",
			input:    123456789,
			expected: 123456789,
		},
		{
			name:     "bot api -100 id",
			input:    -1001234567890,
			expected: 1234567890,
		},
		{
			name:     "negative non -100 id",
			input:    -123,
			expected: -123,
		},
		{
			name:     "zero",
			input:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeChatID(tt.input); got != tt.expected {
				t.Fatalf("normalizeChatID(%d) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
