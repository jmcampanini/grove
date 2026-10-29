package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPromptTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")

	err := checkPromptTerminal(strings.NewReader(""), &bytes.Buffer{}, "pass --force")
	require.EqualError(t, err, "the prompt needs a terminal on stdin; pass --force")

	t.Setenv("TERM", "dumb")
	assert.NoError(t, checkPromptTerminal(strings.NewReader(""), &bytes.Buffer{}, "pass --force"), "the line-based mode needs no terminal")

	t.Setenv("TERM", "")
	err = checkPromptTerminal(strings.NewReader(""), &bytes.Buffer{}, "pass --force")
	require.ErrorContains(t, err, "TERM")
	assert.Contains(t, err.Error(), "pass --force")
}

func TestAccessiblePromptWritesOnlyStderr(t *testing.T) {
	t.Setenv("TERM", "dumb")
	process := exec.Command(os.Args[0], "-test.run=^TestPromptProcess$")
	process.Env = append(os.Environ(), "GROVE_PROMPT_PROCESS=1")
	process.Stdin = strings.NewReader("y\n")
	var stdout, stderr bytes.Buffer
	process.Stdout = &stdout
	process.Stderr = &stderr

	require.NoError(t, process.Run(), stderr.String())
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "Confirm stream routing")
}

func TestPromptProcess(_ *testing.T) {
	if os.Getenv("GROVE_PROMPT_PROCESS") != "1" {
		return
	}
	var confirmed bool
	form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Confirm stream routing").Value(&confirmed)))
	if err := runPrompt(form, render.ColorNever, "pass --force"); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !confirmed {
		os.Exit(1)
	}
	os.Exit(0)
}
