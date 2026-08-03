package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newIssuePreviewCmd() *cobra.Command {
	var colorMode string
	var fzf bool

	cmd := &cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIssuePreview(cmd, args, colorMode, fzf)
		},
	}
	cmd.Flags().StringVar(&colorMode, "color", "auto", "Color output: auto, always, never")
	cmd.Flags().BoolVar(&fzf, "fzf", false, "Print errors to stdout instead of returning error (for fzf preview)")
	return cmd
}

func runIssuePreview(cmd *cobra.Command, args []string, colorMode string, fzf bool) error {
	handleError := func(err error) error {
		return handlePreviewError(cmd, err, fzf)
	}

	theme, err := resolvePreviewTheme(colorMode, cmd.InOrStdin(), cmd.OutOrStdout())
	if err != nil {
		return handleError(err)
	}

	issueNum, err := strconv.Atoi(args[0])
	if err != nil {
		return handleError(fmt.Errorf("invalid issue number: %s", args[0]))
	}

	rt, err := loadCommandRuntime(cmd)
	if err != nil {
		return handleError(err)
	}

	gh, err := rt.newCachedGitHubClient()
	if err != nil {
		return handleError(err)
	}

	issueInfo, err := gh.GetIssue(issueNum)
	if err != nil {
		return handleError(err)
	}

	w := cmd.OutOrStdout()
	width := detectPreviewWidth(w)

	renderer := &previewRenderer{logger: rt.logger, theme: theme}
	return renderer.renderIssuePreview(w, issueInfo, width)
}
