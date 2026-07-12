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

var (
	prPreviewColorFlag       string
	prPreviewFzfFlag         bool
	previewHasDarkBackground = true
)

var prPreviewCmd = &cobra.Command{
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
	RunE: runPRPreview,
}

func init() {
	prPreviewCmd.Flags().StringVar(&prPreviewColorFlag, "color", "auto", "Color output: auto, always, never")
	prPreviewCmd.Flags().BoolVar(&prPreviewFzfFlag, "fzf", false, "Print errors to stdout instead of returning error (for fzf preview)")
	prCmd.AddCommand(prPreviewCmd)
}

func handlePreviewError(cmd *cobra.Command, err error) error {
	if prPreviewFzfFlag {
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

func runPRPreview(cmd *cobra.Command, args []string) error {
	if err := applyColorMode(prPreviewColorFlag); err != nil {
		return handlePreviewError(cmd, err)
	}

	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return handlePreviewError(cmd, fmt.Errorf("invalid PR number: %s", args[0]))
	}

	rt, err := loadCommandRuntime(cmd.Context())
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	gh, err := rt.newCachedGitHubClient()
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	pr, err := gh.GetPullRequest(prNum)
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	owner, repo, err := github.ParseRepoFromURL(pr.URL)
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	threads, timeline, err := gh.GetPullRequestActivity(owner, repo, prNum)
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	fileComments := make(map[string]int, len(threads))
	for _, t := range threads {
		fileComments[t.Path] += t.CommentCount
	}

	w := cmd.OutOrStdout()
	width := detectPreviewWidth()

	return renderPreview(w, pr, fileComments, timeline, width, prPreviewColorFlag)
}
