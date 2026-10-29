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

// checkPromptTerminal fails when a prompt flow cannot run on the given
// streams: the stream it reads keys from and the stream it draws on must both
// be terminals, because the prompt owns the cursor and the keyboard while it
// runs. The accessible mode huh selects when TERM=dumb reads lines and
// writes plain text, so it needs no terminal. Cursor control also requires
// a nonempty TERM. guidance completes the diagnostic with the way around
// the prompt.
func checkPromptTerminal(in io.Reader, drawOn io.Writer, guidance string) error {
	switch os.Getenv("TERM") {
	case "dumb":
		return nil
	case "":
		return fmt.Errorf("the prompt needs TERM set for cursor control; set TERM=dumb for a line-based prompt, or %s", guidance)
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
// titles too, so both modes write to stderr.
func runPrompt(form *huh.Form, mode render.ColorMode, guidance string) error {
	drawOn := os.Stderr
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
