package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPromptTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")

	err := checkPromptTerminal(strings.NewReader(""), &bytes.Buffer{}, "pass --force")
	require.EqualError(t, err, "the prompt needs a terminal on stdin; pass --force")

	t.Setenv("TERM", "dumb")
	assert.NoError(t, checkPromptTerminal(strings.NewReader(""), &bytes.Buffer{}, "pass --force"), "the line-based mode needs no terminal")
}
