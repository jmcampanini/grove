package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/spf13/cobra"
)

var prPreviewFzfFlag bool

var prPreviewCmd = &cobra.Command{
	Use:   "preview [number]",
	Short: "Show pull request details",
	Long: `Show detailed information about a pull request.

Displays PR metadata (title, author, branch, state), a list of changed files
with additions/deletions counts, and the PR body.

With --fzf, errors are printed to stdout instead of returning an error code,
making it suitable for use in fzf preview panes.`,
	Args: cobra.ExactArgs(1),
	RunE: runPRPreview,
}

func init() {
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

func runPRPreview(cmd *cobra.Command, args []string) error {
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

	return outputPRPreview(cmd.OutOrStdout(), pr, files)
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
