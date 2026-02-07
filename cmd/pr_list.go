package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/dustin/go-humanize"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/jmcampanini/grove-cli/internal/pr"
	"github.com/spf13/cobra"
)

var prListFzfFlag bool

var prListCmd = &cobra.Command{
	Use:   "list",
	Short: "List open pull requests",
	Long: `List open pull requests for the current repository.

By default, outputs a formatted table with PR details.

With --fzf, outputs tab-separated format suitable for fzf integration:
  <number>\t<searchable>\t<display>

The "Local" column shows a checkmark when a worktree exists for the PR.`,
	Args: cobra.NoArgs,
	RunE: runPRList,
}

func init() {
	prListCmd.Flags().BoolVar(&prListFzfFlag, "fzf", false, "Output in fzf-compatible format")
	prCmd.AddCommand(prListCmd)
}

func runPRList(cmd *cobra.Command, _ []string) error {
	rt, err := loadCommandRuntime()
	if err != nil {
		return err
	}
	cfg := rt.cfg

	gh := rt.newGitHubClient()
	if err := gh.Validate(); err != nil {
		return err
	}

	namer, err := naming.NewPullRequestNamer(cfg.PullRequest, cfg.Slugify)
	if err != nil {
		return fmt.Errorf("failed to create PR namer: %w", err)
	}

	query := github.PRQuery{State: github.PRStateOpen}
	prs, err := gh.ListPullRequests(query, github.DefaultPRLimit)
	if err != nil {
		return fmt.Errorf("failed to list pull requests: %w", err)
	}

	worktrees, err := rt.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	matcher := pr.NewMatcher(namer)
	matches := matcher.MatchAll(prs, worktrees)

	if prListFzfFlag {
		return outputPRListFzf(cmd.OutOrStdout(), matches)
	}
	return outputPRListTable(cmd.OutOrStdout(), matches)
}

func outputPRListTable(w io.Writer, matches []pr.Match) error {
	if len(matches) == 0 {
		_, err := fmt.Fprintln(w, "No open pull requests found.")
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
		if match.HasWorktree {
			localMarker = "\u2713" // checkmark
		}

		state := strings.ToLower(string(match.PR.State))
		updated := humanize.Time(match.PR.UpdatedAt)

		rows[i] = []string{
			fmt.Sprintf("%d", match.PR.Number),
			truncateString(match.PR.Title, 40),
			match.PR.AuthorLogin,
			truncateString(match.PR.BranchName, 30),
			state,
			localMarker,
			updated,
		}
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(purple)).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return headerStyle
			case row%2 == 0:
				return evenRowStyle
			default:
				return oddRowStyle
			}
		}).
		Headers("#", "Title", "Author", "Branch", "State", "Local", "Updated").
		Rows(rows...)

	_, err := fmt.Fprintln(w, t)
	return err
}

// Format: <number>\t<searchable>\t<display>
func outputPRListFzf(w io.Writer, matches []pr.Match) error {
	for _, match := range matches {
		number := fmt.Sprintf("%d", match.PR.Number)

		state := strings.ToLower(string(match.PR.State))
		searchable := sanitizeFzfField(fmt.Sprintf("%d %s %s %s %s",
			match.PR.Number,
			match.PR.Title,
			match.PR.BranchName,
			match.PR.AuthorLogin,
			state,
		))

		localPrefix := ""
		if match.HasWorktree {
			localPrefix = "\u2713 " // checkmark with space
		}
		display := sanitizeFzfField(fmt.Sprintf("%s#%d %s [%s] %s",
			localPrefix,
			match.PR.Number,
			match.PR.Title,
			match.PR.AuthorLogin,
			match.PR.BranchName,
		))

		_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", number, searchable, display)
		if err != nil {
			return err
		}
	}
	return nil
}

func sanitizeFzfField(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
