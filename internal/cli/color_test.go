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

// TestStyledStdoutCarriesTheColorFlag checks the wiring from the flag to the
// styled writer; the resolution rules themselves are tested in render.
func TestStyledStdoutCarriesTheColorFlag(t *testing.T) {
	root := newTestRootCommand()
	require.NoError(t, root.PersistentFlags().Set(colorFlag, "always"))

	w, ok := styledStdout(root).(*colorprofile.Writer)

	require.True(t, ok)
	assert.Equal(t, colorprofile.ANSI256, w.Profile)
}
