package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var previewHasDarkBackground = true

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

func detectPreviewWidth() int {
	if cols := os.Getenv("FZF_PREVIEW_COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}

func applyColorMode(mode string) error {
	var profile colorprofile.Profile
	switch mode {
	case "auto":
		profile = colorprofile.Detect(os.Stdout, os.Environ())
	case "always":
		profile = colorprofile.ANSI256
	case "never":
		profile = colorprofile.NoTTY
	default:
		return fmt.Errorf("invalid color mode %q; valid modes: auto, always, never", mode)
	}

	lipgloss.Writer.Profile = profile
	previewHasDarkBackground = detectPreviewHasDarkBackground(profile)
	return nil
}

func detectPreviewHasDarkBackground(profile colorprofile.Profile) bool {
	if dark, ok := detectDarkBackgroundFromEnv(); ok {
		return dark
	}
	if profile <= colorprofile.ASCII || !previewCanQueryBackground() {
		return true
	}
	return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
}

func previewCanQueryBackground() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func previewColorsDisabled() bool {
	return lipgloss.Writer.Profile <= colorprofile.ASCII
}

func writePreviewLine(w io.Writer, s string) error {
	profiled := colorprofile.Writer{Forward: w, Profile: lipgloss.Writer.Profile}
	_, err := fmt.Fprintln(&profiled, s)
	return err
}

func runPRPreview(cmd *cobra.Command, args []string, colorMode string, fzf bool) error {
	handleError := func(err error) error {
		return handlePreviewError(cmd, err, fzf)
	}

	if err := applyColorMode(colorMode); err != nil {
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
	width := detectPreviewWidth()

	return renderPreview(w, pr, fileComments, timeline, width, colorMode)
}
