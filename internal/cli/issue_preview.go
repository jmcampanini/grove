package cli

import (
	"fmt"
	"strconv"

	"github.com/jmcampanini/grove/internal/render"
	"github.com/spf13/cobra"
)

func newIssuePreviewCmd(services commandServices) *cobra.Command {
	var colorMode string
	var fzf bool

	cmd := &cobra.Command{
		Use:   "preview <number>",
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
			return services.runIssuePreview(cmd, args, colorMode, fzf)
		},
	}
	cmd.Flags().StringVar(&colorMode, "color", "auto", "Color output: auto, always, never")
	cmd.Flags().BoolVar(&fzf, "fzf", false, "Print errors to stdout instead of returning error (for fzf preview)")
	return cmd
}

func (services commandServices) runIssuePreview(cmd *cobra.Command, args []string, colorMode string, fzf bool) error {
	handleError := func(err error) error {
		return handlePreviewError(cmd, err, fzf)
	}

	theme, err := render.ResolvePreviewTheme(colorMode, cmd.InOrStdin(), cmd.OutOrStdout())
	if err != nil {
		return handleError(err)
	}

	issueNum, err := strconv.Atoi(args[0])
	if err != nil {
		return handleError(fmt.Errorf("invalid issue number: %s", args[0]))
	}

	rt, err := services.loadRuntime(cmd)
	if err != nil {
		return handleError(err)
	}

	gh, err := rt.NewCachedGitHubClient()
	if err != nil {
		return handleError(err)
	}

	issueInfo, err := gh.GetIssue(issueNum)
	if err != nil {
		return handleError(err)
	}

	w := cmd.OutOrStdout()
	width := render.DetectPreviewWidth(w)

	renderer := render.New(rt.Logger, theme)
	return renderer.Issue(w, issueInfo, width)
}
