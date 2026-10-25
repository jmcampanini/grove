package cli

import (
	"fmt"
	"strconv"

	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/spf13/cobra"
)

func newPRPreviewCmd(services commandServices) *cobra.Command {
	var colorMode string
	var fzf bool

	cmd := &cobra.Command{
		Use:   "preview <number>",
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
			return services.runPRPreview(cmd, args, colorMode, fzf)
		},
	}
	cmd.Flags().StringVar(&colorMode, "color", "auto", "Color output: auto, always, never")
	cmd.Flags().BoolVar(&fzf, "fzf", false, "Print errors to stdout instead of returning error (for fzf preview)")
	return cmd
}

func (services commandServices) runPRPreview(cmd *cobra.Command, args []string, colorMode string, fzf bool) error {
	handleError := func(err error) error {
		return handlePreviewError(cmd, err, fzf)
	}

	theme, err := render.ResolvePreviewTheme(colorMode, cmd.InOrStdin(), cmd.OutOrStdout())
	if err != nil {
		return handleError(err)
	}

	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return handleError(fmt.Errorf("invalid PR number: %s", args[0]))
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

	renderer := render.New(rt.Logger, theme)
	return renderer.PullRequest(w, pr, fileComments, timeline, width)
}
