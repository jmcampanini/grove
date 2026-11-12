package render

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
)

// ColorMode is the --color setting: how styled output decides whether a
// stream gets color.
type ColorMode string

const (
	// ColorAuto detects color per destination stream and honors NO_COLOR.
	ColorAuto ColorMode = "auto"
	// ColorAlways forces color on every styled stream, overriding detection
	// and NO_COLOR.
	ColorAlways ColorMode = "always"
	// ColorNever disables color on every styled stream, overriding detection
	// and NO_COLOR.
	ColorNever ColorMode = "never"
)

// ColorModes lists the accepted --color values in help order.
var ColorModes = []ColorMode{ColorAuto, ColorAlways, ColorNever}

// ParseColorMode returns the mode named by s, or an error naming the valid
// values when s is not one of them.
func ParseColorMode(s string) (ColorMode, error) {
	for _, mode := range ColorModes {
		if s == string(mode) {
			return mode, nil
		}
	}
	names := make([]string, len(ColorModes))
	for i, mode := range ColorModes {
		names[i] = string(mode)
	}
	return "", fmt.Errorf("valid values are %s", strings.Join(names, ", "))
}

// ResolveProfile returns the color profile styled output uses on w. In auto
// mode the profile is detected for w alone: a stream that is not a terminal
// or a TERM=dumb terminal gets no color, and any nonempty NO_COLOR drops the
// detected profile to text attributes only. always and never ignore both the
// detection and NO_COLOR.
func ResolveProfile(mode ColorMode, w io.Writer) colorprofile.Profile {
	switch mode {
	case ColorAlways:
		return colorprofile.ANSI256
	case ColorNever:
		return colorprofile.NoTTY
	default:
		if !IsTerminal(w) || os.Getenv("TERM") == "dumb" {
			return colorprofile.NoTTY
		}
		return withNoColor(colorprofile.Detect(w, os.Environ()), os.Getenv("NO_COLOR"))
	}
}

// withNoColor applies the NO_COLOR rule to a detected profile: any nonempty
// value disables color while keeping text attributes.
func withNoColor(profile colorprofile.Profile, noColor string) colorprofile.Profile {
	if noColor != "" && profile > colorprofile.ASCII {
		return colorprofile.ASCII
	}
	return profile
}

// IsTerminal reports whether stream is a file attached to a terminal.
func IsTerminal(stream any) bool {
	f, ok := stream.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// StyledWriter wraps w so that styled text written through it is downgraded
// to the profile ResolveProfile selects for w: color is dropped or reduced,
// and a stream without color also loses hyperlinks.
func StyledWriter(mode ColorMode, w io.Writer) io.Writer {
	return &colorprofile.Writer{Forward: w, Profile: ResolveProfile(mode, w)}
}
