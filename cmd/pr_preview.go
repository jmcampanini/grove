package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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
	switch mode {
	case "auto":
		return nil
	case "always":
		lipgloss.SetColorProfile(termenv.ANSI256)
		return nil
	case "never":
		lipgloss.SetColorProfile(termenv.Ascii)
		return nil
	default:
		return fmt.Errorf("invalid color mode %q; valid modes: auto, always, never", mode)
	}
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

	gh := rt.newGitHubClient()
	if err := gh.Validate(); err != nil {
		return handlePreviewError(cmd, err)
	}

	pr, err := gh.GetPullRequest(prNum)
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	files, err := gh.GetPullRequestFiles(prNum)
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	threads, err := gh.GetPullRequestReviewThreads(prNum)
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	timeline, err := gh.GetPullRequestTimeline(prNum)
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	fileComments := make(map[string]int, len(threads))
	for _, t := range threads {
		fileComments[t.Path] += t.CommentCount
	}

	w := cmd.OutOrStdout()
	width := detectPreviewWidth()

	return renderPreview(w, pr, files, fileComments, timeline, width, prPreviewColorFlag)
}
