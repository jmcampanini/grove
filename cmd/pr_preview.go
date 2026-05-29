package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/colorprofile"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	prPreviewColorFlag string
	prPreviewFzfFlag   bool
)

var prPreviewCmd = &cobra.Command{
	Use:   "preview [number]",
	Short: "Show pull request details",
	Long: `Show detailed information about a pull request.

Displays PR metadata (title, author, branch, state), CI checks, review status,
high-activity files, all changed files with additions/deletions counts, the PR
body rendered as markdown, and an activity timeline.

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
	// When stdout isn't a TTY (fzf preview, piped output), Lip Gloss can't query
	// the terminal for its background color and defaults to dark. Fall back to
	// COLORFGBG env var so adaptive colors pick the right variant.
	fi, err := os.Stdout.Stat()
	if err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		if dark, ok := detectDarkBackgroundFromEnv(); ok {
			compat.HasDarkBackground = dark
		}
	}

	switch mode {
	case "auto":
		setPreviewColorProfile(colorprofile.Detect(os.Stdout, os.Environ()))
		return nil
	case "always":
		setPreviewColorProfile(colorprofile.ANSI256)
		return nil
	case "never":
		setPreviewColorProfile(colorprofile.ASCII)
		return nil
	default:
		return fmt.Errorf("invalid color mode %q; valid modes: auto, always, never", mode)
	}
}

func setPreviewColorProfile(profile colorprofile.Profile) {
	lipgloss.Writer.Profile = profile
	compat.Profile = profile
}

func previewColorsDisabled() bool {
	return lipgloss.Writer.Profile <= colorprofile.ASCII
}

func writePreviewLine(w io.Writer, s string) error {
	profiled := colorprofile.Writer{Forward: w, Profile: lipgloss.Writer.Profile}
	_, err := fmt.Fprintln(&profiled, s)
	return err
}

func detectDarkBackgroundFromEnv() (dark bool, ok bool) {
	parts := strings.Split(os.Getenv("COLORFGBG"), ";")
	if len(parts) < 2 {
		return false, false
	}
	bg, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return false, false
	}
	return bg < 7 || bg == 8, true
}

func runPRPreview(cmd *cobra.Command, args []string) error {
	if err := applyColorMode(prPreviewColorFlag); err != nil {
		return handlePreviewError(cmd, err)
	}

	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return handlePreviewError(cmd, fmt.Errorf("invalid PR number: %s", args[0]))
	}

	rt, err := loadCommandRuntime()
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
