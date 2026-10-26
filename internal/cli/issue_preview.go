package cli

import (
	"github.com/jmcampanini/grove/internal/render"
	"github.com/spf13/cobra"
)

func newIssuePreviewCmd(services commandServices) *cobra.Command {
	var fzf bool

	cmd := &cobra.Command{
		Use:   "preview <number>",
		Short: "Show issue details",
		Long: `Show an issue on stdout: title, author, state, labels, assignees, and
milestone, the body rendered as Markdown, and the most recent comments.
The output is styled per --color and sized to the fzf preview pane
(FZF_PREVIEW_COLUMNS) or the terminal width; it is not a machine-readable
format. Nothing is modified.

` + helpGH + `

Security note: Markdown links in the issue body and comments may be rendered
as clickable terminal hyperlinks. Only open links from issues/authors you trust.

` + helpFzfPreview + `

` + helpFailure,
		Example: `  grove issue list --fzf | fzf --delimiter '\t' --with-nth 3 \
    --preview 'grove issue preview --fzf {1}'`,
		Args: numberArgs("issue"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return services.runIssuePreview(cmd, args, fzf)
		},
	}
	cmd.Flags().BoolVar(&fzf, "fzf", false, "Print a failure on stdout and exit 0 (fzf preview pane mode)")
	return cmd
}

func (services commandServices) runIssuePreview(cmd *cobra.Command, args []string, fzf bool) error {
	handleError := func(err error) error {
		return handlePreviewError(cmd, err, fzf)
	}

	issueNum, err := parseNumberOperand(args[0])
	if err != nil {
		return handleError(err)
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
	theme := render.ResolvePreviewTheme(colorMode(cmd), cmd.InOrStdin(), w)

	renderer := render.New(rt.Logger, theme)
	return renderer.Issue(w, issueInfo, width)
}
