package cli

import (
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/spf13/cobra"
)

func newPRPreviewCmd(services commandServices) *cobra.Command {
	var fzf bool

	cmd := &cobra.Command{
		Use:   "preview <number>",
		Short: "Show pull request details",
		Long: `Show a pull request on stdout: title, author, branch, and state, CI
checks, review status, high-activity files, every changed file with its
addition and deletion counts, the body rendered as Markdown, and an
activity timeline. The output is styled per --color and sized to the fzf
preview pane (FZF_PREVIEW_COLUMNS) or the terminal width; it is not a
machine-readable format. Nothing is modified.

` + helpGH + `

Security note: Markdown links in the PR body may be rendered as
clickable terminal hyperlinks. Only open links from PRs/authors you trust.

` + helpFzfPreview + `

` + helpFailure,
		Example: `  grove pr list --fzf | fzf --delimiter '\t' --with-nth 3 \
    --preview 'grove pr preview --fzf {1}'`,
		Args: numberArgs("pull request"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return services.runPRPreview(cmd, args, fzf)
		},
	}
	cmd.Flags().BoolVar(&fzf, "fzf", false, "Print a failure on stdout and exit 0 (fzf preview pane mode)")
	return cmd
}

func (services commandServices) runPRPreview(cmd *cobra.Command, args []string, fzf bool) error {
	handleError := func(err error) error {
		return handlePreviewError(cmd, err, fzf)
	}

	prNum, err := parseNumberOperand(args[0])
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
	width := render.DetectPreviewWidth(w)
	theme := render.ResolvePreviewTheme(colorMode(cmd), cmd.InOrStdin(), w)

	renderer := render.New(rt.Logger, theme)
	return renderer.PullRequest(w, pr, fileComments, timeline, width)
}
