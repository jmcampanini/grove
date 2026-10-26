package render

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
)

// Theme is the per-execution color state for preview rendering,
// resolved from the --color mode and the injected streams.
type Theme struct {
	hasDarkBackground bool
	profile           colorprofile.Profile
}

func (t Theme) colorsDisabled() bool {
	return t.profile <= colorprofile.ASCII
}

// Renderer renders PR and issue previews with a fixed theme and
// diagnostic logger for one execution.
type Renderer struct {
	logger *log.Logger
	theme  Theme
}

// DetectPreviewWidth uses the fzf pane width, terminal width, or an 80-column fallback.
func DetectPreviewWidth(out io.Writer) int {
	if cols := os.Getenv("FZF_PREVIEW_COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	if f, ok := out.(*os.File); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			return w
		}
	}
	return 80
}

// ResolvePreviewTheme derives the color profile and background for one
// execution from the --color mode and the injected streams. The profile
// follows ResolveProfile for out; the background follows HasDarkBackground,
// so a stream that is not a terminal gets no color and the dark default.
func ResolvePreviewTheme(mode ColorMode, in io.Reader, out io.Writer) Theme {
	profile := ResolveProfile(mode, out)
	return Theme{
		hasDarkBackground: HasDarkBackground(profile, in, out),
		profile:           profile,
	}
}

func (r *Renderer) writePreviewLine(w io.Writer, s string) error {
	profiled := colorprofile.Writer{Forward: w, Profile: r.theme.profile}
	_, err := fmt.Fprintln(&profiled, s)
	return err
}

// New creates a renderer with a fixed theme and diagnostic logger.
func New(logger *log.Logger, theme Theme) *Renderer {
	return &Renderer{logger: logger, theme: theme}
}
