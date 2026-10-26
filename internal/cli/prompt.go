package cli

import (
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jmcampanini/grove/internal/render"
)

// promptOutput returns the stream a huh prompt flow draws on: os.Stderr,
// or os.Stdout in the line-based accessible mode huh selects when
// TERM=dumb. Every prompt reads os.Stdin.
func promptOutput() io.Writer {
	if os.Getenv("TERM") == "dumb" {
		return os.Stdout
	}
	return os.Stderr
}

// checkPromptTerminal fails when a prompt flow cannot run on the given
// streams: the stream it reads keys from and the stream it draws on must both
// be terminals, because the prompt owns the cursor and the keyboard while it
// runs. The accessible mode huh selects when TERM=dumb reads lines and
// writes plain text, so it needs no terminal. Cursor control is inferred
// deliberately: the libraries in use offer no probe for it, and TERM=dumb
// is the one terminal that reports it absent. guidance completes the
// diagnostic with the way around the prompt.
func checkPromptTerminal(in io.Reader, drawOn io.Writer, guidance string) error {
	if os.Getenv("TERM") == "dumb" {
		return nil
	}
	if !render.IsTerminal(in) {
		return fmt.Errorf("the prompt needs a terminal on stdin; %s", guidance)
	}
	if !render.IsTerminal(drawOn) {
		return fmt.Errorf("the prompt needs a terminal on stderr; %s", guidance)
	}
	return nil
}

// runPrompt checks the terminal a prompt flow needs, then runs the form with
// the color profile the --color mode resolves for the stream it draws on.
// The profile never drops below ASCII: the no-color profile would also strip
// the cursor control the prompt is made of. The accessible mode styles its
// titles too, so its output goes through the writer the mode selects.
func runPrompt(form *huh.Form, mode render.ColorMode, guidance string) error {
	drawOn := promptOutput()
	if err := checkPromptTerminal(os.Stdin, drawOn, guidance); err != nil {
		return err
	}
	if os.Getenv("TERM") == "dumb" {
		return form.WithOutput(render.StyledWriter(mode, drawOn)).Run()
	}
	profile := max(render.ResolveProfile(mode, drawOn), colorprofile.ASCII)
	// WithProgramOptions replaces huh's defaults, so the output stream is
	// set again alongside the profile.
	form = form.WithProgramOptions(tea.WithOutput(drawOn), tea.WithColorProfile(profile))
	return form.Run()
}
