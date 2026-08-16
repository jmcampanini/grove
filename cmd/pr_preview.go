package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// previewTheme is the per-execution color state for preview rendering,
// resolved from the --color flag and the injected streams.
type previewTheme struct {
	hasDarkBackground bool
	profile           colorprofile.Profile
}

func (t previewTheme) colorsDisabled() bool {
	return t.profile <= colorprofile.ASCII
}

// previewRenderer renders PR and issue previews with a fixed theme and
// diagnostic logger for one execution.
type previewRenderer struct {
	logger *log.Logger
	theme  previewTheme
}

func newPRPreviewCmd() *cobra.Command {
	var colorMode string
	var fzf bool

	cmd := &cobra.Command{
		Use:   "preview [number]",
		Short: "Show pull request details",
		Long: `Show detailed information about a pull request.

Displays PR metadata (title, author, branch, state), CI checks, review status,
high-activity files, all changed files with additions/deletions counts, the PR
body rendered as markdown, and an activity timeline.

Security note: Markdown links in the PR body may be rendered as
clickable terminal hyperlinks. Only open links from PRs/authors you trust.

With --fzf, errors are printed to stdout instead of returning an error code,
making it suitable for use in fzf preview panes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPRPreview(cmd, args, colorMode, fzf)
		},
	}
	cmd.Flags().StringVar(&colorMode, "color", "auto", "Color output: auto, always, never")
	cmd.Flags().BoolVar(&fzf, "fzf", false, "Print errors to stdout instead of returning error (for fzf preview)")
	return cmd
}

func handlePreviewError(cmd *cobra.Command, err error, fzf bool) error {
	if fzf {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Error: %v\n", err)
		return nil
	}
	return err
}

func detectPreviewWidth(out io.Writer) int {
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

// resolvePreviewTheme derives the color profile and background for one
// execution from the --color mode and the injected streams. Streams that are
// not terminals get the no-color profile and the dark-background default.
func resolvePreviewTheme(mode string, in io.Reader, out io.Writer) (previewTheme, error) {
	var profile colorprofile.Profile
	switch mode {
	case "auto":
		profile = colorprofile.Detect(out, os.Environ())
	case "always":
		profile = colorprofile.ANSI256
	case "never":
		profile = colorprofile.NoTTY
	default:
		return previewTheme{}, fmt.Errorf("invalid color mode %q; valid modes: auto, always, never", mode)
	}

	return previewTheme{
		hasDarkBackground: detectPreviewHasDarkBackground(profile, in, out),
		profile:           profile,
	}, nil
}

func detectPreviewHasDarkBackground(profile colorprofile.Profile, in io.Reader, out io.Writer) bool {
	if dark, ok := detectDarkBackgroundFromEnv(); ok {
		return dark
	}
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	canQuery := inOK && outOK &&
		term.IsTerminal(int(inFile.Fd())) && term.IsTerminal(int(outFile.Fd()))
	if profile <= colorprofile.ASCII || !canQuery {
		return true
	}
	return lipgloss.HasDarkBackground(inFile, outFile)
}

func (r *previewRenderer) writePreviewLine(w io.Writer, s string) error {
	profiled := colorprofile.Writer{Forward: w, Profile: r.theme.profile}
	_, err := fmt.Fprintln(&profiled, s)
	return err
}

func runPRPreview(cmd *cobra.Command, args []string, colorMode string, fzf bool) error {
	handleError := func(err error) error {
		return handlePreviewError(cmd, err, fzf)
	}

	theme, err := resolvePreviewTheme(colorMode, cmd.InOrStdin(), cmd.OutOrStdout())
	if err != nil {
		return handleError(err)
	}

	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return handleError(fmt.Errorf("invalid PR number: %s", args[0]))
	}

	rt, err := loadCommandRuntime(cmd)
	if err != nil {
		return handleError(err)
	}

	gh, err := rt.newCachedGitHubClient()
	if err != nil {
		return handleError(err)
	}

	pr, err := gh.GetPullRequest(prNum)
	if err != nil {
		return handleError(err)
	}

	owner, repo, err := github.ParseRepoFromURL(pr.URL)
	if err != nil {
		return handleError(err)
	}

	threads, timeline, err := gh.GetPullRequestActivity(owner, repo, prNum)
	if err != nil {
		return handleError(err)
	}

	fileComments := make(map[string]int, len(threads))
	for _, t := range threads {
		fileComments[t.Path] += t.CommentCount
	}

	w := cmd.OutOrStdout()
	width := detectPreviewWidth(w)

	renderer := &previewRenderer{logger: rt.logger, theme: theme}
	return renderer.renderPreview(w, pr, fileComments, timeline, width)
}
