package render

import (
	"bytes"
	"fmt"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseColorMode(t *testing.T) {
	for _, mode := range ColorModes {
		got, err := ParseColorMode(string(mode))
		require.NoError(t, err)
		assert.Equal(t, mode, got)
	}

	_, err := ParseColorMode("sometimes")
	require.EqualError(t, err, "valid values are auto, always, never")
}

// TestResolveProfileOnNonTerminal covers the project's own color rules on a
// writer that is not a terminal: auto never colors it, whatever the
// environment asks for, and always and never override both the detection
// and NO_COLOR.
func TestResolveProfileOnNonTerminal(t *testing.T) {
	tests := []struct {
		mode    ColorMode
		name    string
		noColor string
		want    colorprofile.Profile
	}{
		{name: "auto without NO_COLOR", mode: ColorAuto, want: colorprofile.NoTTY},
		{name: "auto with NO_COLOR=1", mode: ColorAuto, noColor: "1", want: colorprofile.NoTTY},
		{name: "always overrides NO_COLOR", mode: ColorAlways, noColor: "1", want: colorprofile.ANSI256},
		{name: "never", mode: ColorNever, want: colorprofile.NoTTY},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CLICOLOR_FORCE", "1")
			t.Setenv("FORCE_COLOR", "3")
			t.Setenv("NO_COLOR", tt.noColor)
			t.Setenv("TERM", "xterm-256color")
			t.Setenv("TTY_FORCE", "1")

			assert.Equal(t, tt.want, ResolveProfile(tt.mode, &bytes.Buffer{}))
		})
	}
}

// TestWithNoColor covers the NO_COLOR rule on a profile detected for a
// terminal: any nonempty value, not only a boolean one, disables color.
func TestWithNoColor(t *testing.T) {
	tests := []struct {
		noColor string
		profile colorprofile.Profile
		want    colorprofile.Profile
	}{
		{noColor: "", profile: colorprofile.TrueColor, want: colorprofile.TrueColor},
		{noColor: "1", profile: colorprofile.ANSI256, want: colorprofile.ASCII},
		{noColor: "x", profile: colorprofile.ANSI256, want: colorprofile.ASCII},
		{noColor: "false", profile: colorprofile.ANSI, want: colorprofile.ASCII},
		{noColor: "foo", profile: colorprofile.NoTTY, want: colorprofile.NoTTY},
	}

	for _, tt := range tests {
		t.Run("NO_COLOR="+tt.noColor, func(t *testing.T) {
			assert.Equal(t, tt.want, withNoColor(tt.profile, tt.noColor))
		})
	}
}

func TestStyledWriterStripsColorOnNonTerminal(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("TTY_FORCE", "")
	styled := lipgloss.NewStyle().Foreground(ColorRed).Render("plain")

	var buf bytes.Buffer
	_, err := fmt.Fprintln(StyledWriter(ColorAuto, &buf), styled)
	require.NoError(t, err)
	assert.Equal(t, "plain\n", buf.String())
}
