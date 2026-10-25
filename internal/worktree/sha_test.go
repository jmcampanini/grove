package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShortSHASafe(t *testing.T) {
	tests := []struct {
		name   string
		sha    string
		maxLen int
		want   string
	}{
		{
			name:   "normal SHA truncated",
			sha:    "abc1234def5678",
			maxLen: 7,
			want:   "abc1234",
		},
		{
			name:   "SHA exactly maxLen",
			sha:    "abc1234",
			maxLen: 7,
			want:   "abc1234",
		},
		{
			name:   "SHA shorter than maxLen",
			sha:    "abc",
			maxLen: 7,
			want:   "abc",
		},
		{
			name:   "empty SHA returns placeholder",
			sha:    "",
			maxLen: 7,
			want:   "(no sha)",
		},
		{
			name:   "maxLen of 0 returns empty",
			sha:    "abc1234",
			maxLen: 0,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShortSHASafe(tt.sha, tt.maxLen)
			assert.Equal(t, tt.want, got)
		})
	}
}
