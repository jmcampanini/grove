package cli

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/dustin/go-humanize"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/issue"
	"github.com/jmcampanini/grove/internal/listformat"
	"github.com/jmcampanini/grove/internal/naming"
	"github.com/spf13/cobra"
)

func newIssueListCmd(services commandServices) *cobra.Command {
	var fzf bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List open issues",
		Long: `List the open issues of the current repository on stdout. Nothing is
modified and nothing prompts.

By default the output is a styled table: number, title, author, labels, a
Local checkmark when a worktree exists for the issue, and when it was
updated. With --fzf each line is <number>\t<searchable>\t<display>, a
machine-readable format that is never styled, whatever --color says.

` + helpGH + `

` + helpFailure,
		Example: `  grove issue list --fzf | fzf --delimiter '\t' --with-nth 3 --accept-nth 1`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return services.runIssueList(cmd, fzf)
		},
	}
	cmd.Flags().BoolVar(&fzf, "fzf", false, "Print <number>\\t<searchable>\\t<display> lines for fzf")
	return cmd
}

func (services commandServices) runIssueList(cmd *cobra.Command, fzf bool) error {
	rt, err := services.loadRuntime(cmd)
	if err != nil {
		return err
	}
	cfg := rt.Config

	gh, err := rt.NewCachedGitHubClient()
	if err != nil {
		return err
	}

	namer, err := naming.NewIssueNamer(cfg.Issue, cfg.Naming)
	if err != nil {
		return fmt.Errorf("failed to create issue namer: %w", err)
	}

	query := github.IssueQuery{State: github.IssueStateOpen}
	issues, err := gh.ListIssues(query, github.DefaultIssueLimit)
	if err != nil {
		return fmt.Errorf("failed to list issues: %w", err)
	}

	worktrees, err := rt.GitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	matcher := issue.NewMatcher(namer, rt.Logger)
	matches := matcher.MatchAll(issues, worktrees)

	if fzf {
		return outputIssueListFzf(cmd.OutOrStdout(), matches)
	}
	return outputIssueListTable(styledStdout(cmd), matches)
}

// maxListLabels is how many label names the list shows before collapsing the
// rest into a "+n" overflow count.
const maxListLabels = 2

func formatIssueLabels(labels []github.Label) string {
	if len(labels) == 0 {
		return ""
	}

	shown := min(len(labels), maxListLabels)
	names := make([]string, 0, shown)
	for _, l := range labels[:shown] {
		names = append(names, l.Name)
	}

	s := strings.Join(names, ", ")
	if extra := len(labels) - shown; extra > 0 {
		s = fmt.Sprintf("%s +%d", s, extra)
	}
	return s
}

func allIssueLabels(labels []github.Label) string {
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.Name)
	}
	return strings.Join(names, ", ")
}

func outputIssueListTable(w io.Writer, matches []issue.Match) error {
	if len(matches) == 0 {
		_, err := fmt.Fprintln(w, "No open issues found.")
		return err
	}

	purple := lipgloss.Color("99")
	gray := lipgloss.Color("245")
	lightGray := lipgloss.Color("241")

	headerStyle := lipgloss.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	oddRowStyle := cellStyle.Foreground(gray)
	evenRowStyle := cellStyle.Foreground(lightGray)

	rows := make([][]string, len(matches))
	for i, match := range matches {
		localMarker := ""
		if match.HasWorktree() {
			localMarker = "✓" // checkmark
		}

		rows[i] = []string{
			fmt.Sprintf("%d", match.Issue.Number),
			listformat.TruncateString(match.Issue.Title, 40),
			match.Issue.AuthorLogin,
			listformat.TruncateString(formatIssueLabels(match.Issue.Labels), 30),
			localMarker,
			humanize.Time(match.Issue.UpdatedAt),
		}
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(purple)).
		StyleFunc(func(row, _ int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return headerStyle
			case row%2 == 0:
				return evenRowStyle
			default:
				return oddRowStyle
			}
		}).
		Headers("#", "Title", "Author", "Labels", "Local", "Updated").
		Rows(rows...)

	_, err := fmt.Fprintln(w, t)
	return err
}

func outputIssueListFzf(w io.Writer, matches []issue.Match) error {
	for _, match := range matches {
		number := fmt.Sprintf("%d", match.Issue.Number)

		state := strings.ToLower(string(match.Issue.State))
		searchable := listformat.SanitizeFzfField(fmt.Sprintf("%d %s %s %s %s",
			match.Issue.Number,
			match.Issue.Title,
			allIssueLabels(match.Issue.Labels),
			match.Issue.AuthorLogin,
			state,
		))

		localPrefix := ""
		if match.HasWorktree() {
			localPrefix = "✓ " // checkmark with space
		}
		display := listformat.SanitizeFzfField(fmt.Sprintf("%s#%d %s [%s] %s",
			localPrefix,
			match.Issue.Number,
			match.Issue.Title,
			match.Issue.AuthorLogin,
			formatIssueLabels(match.Issue.Labels),
		))

		_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", number, searchable, display)
		if err != nil {
			return err
		}
	}
	return nil
}
