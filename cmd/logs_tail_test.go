package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadTail(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		n        int
		expected string
	}{
		{
			name:     "basic last N lines",
			content:  "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n",
			n:        3,
			expected: "line8\nline9\nline10\n",
		},
		{
			name:     "fewer than N lines",
			content:  "line1\nline2\nline3\n",
			n:        10,
			expected: "line1\nline2\nline3\n",
		},
		{
			name:     "empty file",
			content:  "",
			n:        5,
			expected: "",
		},
		{
			name:     "trailing newline",
			content:  "a\nb\nc\n",
			n:        2,
			expected: "b\nc\n",
		},
		{
			name:     "no trailing newline",
			content:  "a\nb\nc",
			n:        2,
			expected: "b\nc",
		},
		{
			name:     "n equals 1",
			content:  "a\nb\nc\n",
			n:        1,
			expected: "c\n",
		},
		{
			name:     "blank lines count",
			content:  "a\n\nb\nc\n",
			n:        2,
			expected: "b\nc\n",
		},
		{
			name:     "exactly N lines",
			content:  "a\nb\nc\n",
			n:        3,
			expected: "a\nb\nc\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.log")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0644))

			f, err := os.Open(path)
			require.NoError(t, err)
			defer func() { _ = f.Close() }()

			got, err := readTail(f, tt.n)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(got))
		})
	}
}

func TestValidateTailLines(t *testing.T) {
	tests := []struct {
		name           string
		n              int
		wantErr        bool
		wantErrContain string
	}{
		{
			name: "valid minimum",
			n:    1,
		},
		{
			name: "valid default",
			n:    25,
		},
		{
			name: "valid maximum",
			n:    10000,
		},
		{
			name:           "zero",
			n:              0,
			wantErr:        true,
			wantErrContain: "must be a positive integer, got 0",
		},
		{
			name:           "negative",
			n:              -5,
			wantErr:        true,
			wantErrContain: "must be a positive integer, got -5",
		},
		{
			name:           "over max",
			n:              10001,
			wantErr:        true,
			wantErrContain: "must be at most 10000, got 10001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTailLines(tt.n)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContain)
				return
			}
			require.NoError(t, err)
		})
	}
}
