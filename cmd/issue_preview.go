package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	issuePreviewColorFlag string
	issuePreviewFzfFlag   bool
)

var issuePreviewCmd = &cobra.Command{
	Use:   "preview [number]",
	Short: "Show issue details",
	Long: `Show detailed information about an issue.

Displays issue metadata (title, author, state, labels, assignees, milestone),
the issue body rendered as markdown, and the most recent comments.

Security note: Markdown links in the issue body and comments may be rendered
as clickable terminal hyperlinks. Only open links from issues/authors you trust.

With --fzf, errors are printed to stdout instead of returning an error code,
making it suitable for use in fzf preview panes.`,
	Args: cobra.ExactArgs(1),
	RunE: runIssuePreview,
}

func init() {
	issuePreviewCmd.Flags().StringVar(&issuePreviewColorFlag, "color", "auto", "Color output: auto, always, never")
	issuePreviewCmd.Flags().BoolVar(&issuePreviewFzfFlag, "fzf", false, "Print errors to stdout instead of returning error (for fzf preview)")
	issueCmd.AddCommand(issuePreviewCmd)
}

func handleIssuePreviewError(cmd *cobra.Command, err error) error {
	if issuePreviewFzfFlag {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Error: %v\n", err)
		return nil
	}
	return err
}

func runIssuePreview(cmd *cobra.Command, args []string) error {
	if err := applyColorMode(issuePreviewColorFlag); err != nil {
		return handleIssuePreviewError(cmd, err)
	}

	issueNum, err := strconv.Atoi(args[0])
	if err != nil {
		return handleIssuePreviewError(cmd, fmt.Errorf("invalid issue number: %s", args[0]))
	}

	rt, err := loadCommandRuntime(cmd.Context())
	if err != nil {
		return handleIssuePreviewError(cmd, err)
	}

	gh, err := rt.newCachedGitHubClient()
	if err != nil {
		return handleIssuePreviewError(cmd, err)
	}

	issueInfo, err := gh.GetIssue(issueNum)
	if err != nil {
		return handleIssuePreviewError(cmd, err)
	}

	w := cmd.OutOrStdout()
	width := detectPreviewWidth()

	return renderIssuePreview(w, issueInfo, width, issuePreviewColorFlag)
}
