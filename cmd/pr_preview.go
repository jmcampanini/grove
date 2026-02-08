package cmd

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	prPreviewColorFlag string
	prPreviewFzfFlag   bool
	prPreviewStyleFlag string
)

var validPreviewStyles = []string{"card", "dashboard", "minimal", "context", "board", "timeline", "review"}

var prPreviewCmd = &cobra.Command{
	Use:   "preview [number]",
	Short: "Show pull request details",
	Long: `Show detailed information about a pull request.

Displays PR metadata (title, author, branch, state), a list of changed files
with additions/deletions counts, and the PR body.

Styles:
  card       Bordered card with sections (Group A)
  dashboard  Compact key-value dashboard (Group A)
  minimal    Minimal GitHub-inspired layout (Group A)
  context    Full context card with CI/reviews (Group B)
  board      Status board with tables (Group B)
  timeline   Activity timeline (Group B)
  review     Full review with markdown body and activity

With --fzf, errors are printed to stdout instead of returning an error code,
making it suitable for use in fzf preview panes.`,
	Args: cobra.ExactArgs(1),
	RunE: runPRPreview,
}

var validColorModes = []string{"auto", "always", "never"}

func init() {
	prPreviewCmd.Flags().StringVar(&prPreviewColorFlag, "color", "auto", "Color output: auto, always, never")
	prPreviewCmd.Flags().BoolVar(&prPreviewFzfFlag, "fzf", false, "Print errors to stdout instead of returning error (for fzf preview)")
	prPreviewCmd.Flags().StringVar(&prPreviewStyleFlag, "style", "review", "Preview style: card, dashboard, minimal, context, board, timeline, review")
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
	if !slices.Contains(validColorModes, mode) {
		return fmt.Errorf("invalid color mode %q; valid modes: %s", mode, strings.Join(validColorModes, ", "))
	}
	switch mode {
	case "always":
		lipgloss.SetColorProfile(termenv.ANSI256)
	case "never":
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	return nil
}

func runPRPreview(cmd *cobra.Command, args []string) error {
	style := prPreviewStyleFlag
	if !slices.Contains(validPreviewStyles, style) {
		return handlePreviewError(cmd, fmt.Errorf("invalid style %q; valid styles: %s", style, strings.Join(validPreviewStyles, ", ")))
	}

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

	w := cmd.OutOrStdout()
	width := detectPreviewWidth()

	switch style {
	case "card":
		return renderCard(w, pr, files, width)
	case "dashboard":
		return renderDashboard(w, pr, files, width)
	case "minimal":
		return renderMinimal(w, pr, files, width)
	case "context":
		return renderContext(w, pr, files, width)
	case "board":
		return renderBoard(w, pr, files, width)
	case "timeline":
		return renderTimeline(w, pr, files, width)
	case "review":
		return runReviewStyle(cmd, w, gh, pr, files, prNum, width)
	default:
		return renderContext(w, pr, files, width)
	}
}

func runReviewStyle(cmd *cobra.Command, w io.Writer, gh github.GitHub, pr github.PullRequest, files []github.PullRequestFile, prNum, width int) error {
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

	return renderReview(w, pr, files, fileComments, timeline, width, prPreviewColorFlag)
}

func outputPRPreview(w io.Writer, pr github.PullRequest, files []github.PullRequestFile) error {
	var sb strings.Builder

	fmt.Fprintf(&sb, "PR #%d\n", pr.Number)
	sb.WriteString(strings.Repeat("\u2500", 29))
	sb.WriteString("\n")

	fmt.Fprintf(&sb, "Title:  %s\n", pr.Title)
	fmt.Fprintf(&sb, "Author: %s\n", pr.AuthorLogin)
	fmt.Fprintf(&sb, "Branch: %s\n", pr.BranchName)
	fmt.Fprintf(&sb, "State:  %s\n", strings.ToLower(string(pr.State)))
	sb.WriteString("\n")

	const maxFiles = 30
	fmt.Fprintf(&sb, "Files changed (%d):\n", pr.FilesChanged)

	displayCount := min(len(files), maxFiles)

	for _, f := range files[:displayCount] {
		fmt.Fprintf(&sb, "  %s (+%d, -%d)\n", f.Path, f.Additions, f.Deletions)
	}

	if remaining := pr.FilesChanged - displayCount; remaining > 0 {
		fmt.Fprintf(&sb, "  (and %d more files...)\n", remaining)
	}

	sb.WriteString("\n")
	sb.WriteString(pr.Body)
	sb.WriteString("\n")

	_, err := fmt.Fprint(w, sb.String())
	return err
}
