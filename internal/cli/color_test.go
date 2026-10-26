package cli

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColorFlagParsesEveryModeAndRejectsTheRest(t *testing.T) {
	for _, want := range render.ColorModes {
		root := newTestRootCommand()
		require.NoError(t, root.PersistentFlags().Set(colorFlag, string(want)))
		assert.Equal(t, want, colorMode(root))
	}

	root := newTestRootCommand()
	assert.Equal(t, render.ColorAuto, colorMode(root), "auto is the default")
	err := root.PersistentFlags().Set(colorFlag, "x")
	require.EqualError(t, err, `invalid argument "x" for "--color" flag: valid values are auto, always, never`)
}

// TestStyledStdoutHonorsColorFlagOnNonTerminal checks the resolution on a
// buffer, the one destination tests can own: auto and never strip color, and
// always keeps it even with NO_COLOR set.
func TestStyledStdoutHonorsColorFlagOnNonTerminal(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "yes")
	t.Setenv("TTY_FORCE", "")

	for _, tt := range []struct {
		mode render.ColorMode
		want colorprofile.Profile
	}{
		{mode: render.ColorAuto, want: colorprofile.NoTTY},
		{mode: render.ColorNever, want: colorprofile.NoTTY},
		{mode: render.ColorAlways, want: colorprofile.ANSI256},
	} {
		root := newTestRootCommand()
		require.NoError(t, root.PersistentFlags().Set(colorFlag, string(tt.mode)))
		w, ok := styledStdout(root).(*colorprofile.Writer)
		require.True(t, ok)
		assert.Equalf(t, tt.want, w.Profile, "--color=%s", tt.mode)
	}
}
