package listformat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFzfField(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no special characters",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "tabs replaced with spaces",
			input: "hello\tworld",
			want:  "hello world",
		},
		{
			name:  "newlines replaced with spaces",
			input: "hello\nworld",
			want:  "hello world",
		},
		{
			name:  "carriage returns replaced with spaces",
			input: "hello\rworld",
			want:  "hello world",
		},
		{
			name:  "multiple special characters",
			input: "hello\t\n\rworld",
			want:  "hello   world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFzfField(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "string shorter than maxLen",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "string equal to maxLen",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "string longer than maxLen",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "maxLen less than 3",
			input:  "hello",
			maxLen: 2,
			want:   "he",
		},
		{
			name:   "maxLen exactly 3",
			input:  "hello",
			maxLen: 3,
			want:   "hel",
		},
		{
			name:   "maxLen of 4",
			input:  "hello world",
			maxLen: 4,
			want:   "h...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "multi-byte unicode truncation",
			input:  "héllo wörld",
			maxLen: 8,
			want:   "héllo...",
		},
		{
			name:   "emoji truncation",
			input:  "🎉🎊🎈🎁🎂",
			maxLen: 4,
			want:   "🎉...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.want, got)
		})
	}
}
